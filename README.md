# Activity

AI-powered git commit analyzer that generates human-readable summaries of repository activity. Uses intelligent agents to selectively fetch code diffs only when commit messages are unclear. Deployed as a web application behind an auth proxy.

## Features

- **Intelligent Analysis**: Agent-based analyzer decides when to fetch diffs vs. using commit messages
- **Weekly Reports**: Generate week-indexed summaries with backfill support
- **Cost Controls**: Hard limits on diff fetching, diff size, and total tokens
- **Incremental Tracking**: Analyzes only new commits since last run
- **Multi-Repository**: Track and analyze multiple repositories
- **Newsletter**: Email subscribers with activity digests via SendGrid
- **Admin Interface**: Manage repositories, subscribers, and admins through the web UI
- **Auth Proxy Integration**: Admin routes protected via configurable auth header

## Requirements

- Go 1.25.3+
- PostgreSQL
- Google Gemini API key
- Git repositories to analyze

## Installation

```bash
go install github.com/perbu/activity@latest
```

Or build from source:

```bash
git clone https://github.com/perbu/activity
cd activity
go build .
```

## Docker

Docker images are built and pushed to GitHub Container Registry on tagged releases.

```bash
# Pull the latest image
docker pull ghcr.io/perbu/activity:latest

# Run the web server
docker run -p 8080:8080 \
  -v /path/to/data:/data \
  -e GOOGLE_API_KEY=your-api-key \
  -e DATABASE_URL=postgres://user:pass@host:5432/activity?sslmode=disable \
  ghcr.io/perbu/activity:latest \
  --data-dir /data --host 0.0.0.0

# For private GitHub repositories, include GitHub App credentials
docker run -p 8080:8080 \
  -v /path/to/data:/data \
  -e GOOGLE_API_KEY=your-api-key \
  -e DATABASE_URL=postgres://user:pass@host:5432/activity?sslmode=disable \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_INSTALLATION_ID=789012 \
  -e GITHUB_APP_PRIVATE_KEY="$(cat path/to/key.pem)" \
  ghcr.io/perbu/activity:latest \
  --data-dir /data --host 0.0.0.0
```

Available tags:
- `latest` - Latest release
- `1.0.0` - Specific version (from v1.0.0 tag)
- `1.0` - Major.minor version

## Kubernetes

The container image works on Kubernetes without modification. The app exposes `/healthz` for probes and handles `SIGTERM` gracefully (10s drain).

Key resources needed:
- **Deployment** — runs the container, references the ConfigMap, Secret, and PVC
- **Service** — exposes port 8080 within the cluster
- **PersistentVolumeClaim** — stores cloned git repositories (`data-dir`)
- **ConfigMap** — holds `config.yaml`
- **Secret** — holds `GOOGLE_API_KEY` (and optionally `GITHUB_APP_PRIVATE_KEY`, `SENDGRID_API_KEY`)

Example manifests:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: activity-data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: activity-config
data:
  config.yaml: |
    llm:
      provider: gemini
      model: gemini-3.0-flash
      api_key_env: GOOGLE_API_KEY
      use_agent: true
      max_diff_fetches: 5
    web:
      auth_header: oidc-email
      seed_admin: admin@example.com
---
apiVersion: v1
kind: Secret
metadata:
  name: activity-secrets
stringData:
  GOOGLE_API_KEY: "your-api-key"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: activity
spec:
  replicas: 1
  selector:
    matchLabels:
      app: activity
  template:
    metadata:
      labels:
        app: activity
    spec:
      containers:
        - name: activity
          image: ghcr.io/perbu/activity:latest
          args:
            - --config
            - /etc/activity/config.yaml
            - --data-dir
            - /data
            - --host
            - "0.0.0.0"
          ports:
            - containerPort: 8080
          envFrom:
            - secretRef:
                name: activity-secrets
          volumeMounts:
            - name: data
              mountPath: /data
            - name: config
              mountPath: /etc/activity
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 2
            periodSeconds: 5
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: activity-data
        - name: config
          configMap:
            name: activity-config
---
apiVersion: v1
kind: Service
metadata:
  name: activity
spec:
  selector:
    app: activity
  ports:
    - port: 8080
      targetPort: 8080
```

### Single-replica vs. high-availability

The example above runs **one replica** with `strategy: Recreate` and a `ReadWriteOnce` PVC. This is the simplest setup; the tradeoff is a brief downtime (~30s) on every rollout while the old pod releases the volume and the new one attaches. For most installations of a weekly batch tool this is fine.

For zero-downtime rollouts, run **two replicas as a StatefulSet** with `volumeClaimTemplates` so each pod gets its own PVC. Activity uses Postgres advisory-lock leader election to ensure only one replica ever owns the data directory and runs scheduled jobs:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: activity
spec:
  serviceName: activity
  replicas: 2
  selector:
    matchLabels:
      app: activity
  template:
    # ... same pod template as the Deployment example above ...
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 10Gi
```

This produces one PVC per pod (`data-activity-0`, `data-activity-1`) — no `ReadWriteMany` shared volume needed (and not wanted: git is not safe over NFS/CephFS with concurrent writers).

### How leader election works

Both replicas connect to the same Postgres database. On startup each opens a dedicated connection and calls `pg_try_advisory_lock(<key>)`:

- The first instance to acquire the lock holds it on its session for the lifetime of the connection. It runs the scheduled weekly pipeline (repo updates, report generation, newsletter sends).
- The second instance sees the lock held, stays in standby, and re-polls every ~30s.
- When the leader exits — graceful shutdown, crash, or any failure that closes the TCP connection — Postgres releases the session-level lock automatically. On its next poll the standby acquires the lock and takes over.

Implementation lives in `internal/leader`. The lock key (`0x6163746976697479`, ASCII for "activity") is a single namespace shared by all instances pointed at the same database, so no extra coordination is needed.

Reads (dashboard, repo browsing, report viewing) hit the database only and work on either replica. The standby's PVC is unused until that pod becomes leader, at which point the next pipeline run will `git fetch` everything it needs. The current state is visible at `/admin/actions` as a status badge ("Advisory lock held — scheduled jobs enabled" / "Standby — another instance is leader").

> **Caveat:** Manual admin write actions (Update Repos / Generate Reports / Send Newsletters) currently run on whichever replica receives the HTTP request, not just the leader. If you trigger one on the standby it will operate on that pod's (possibly empty or stale) data dir. Until that's gated, send admin writes only to the leader, or stick with the single-replica deployment.

## Running

```bash
# Set required environment variables
export GOOGLE_API_KEY=your-api-key
export DATABASE_URL=postgres://user:pass@localhost:5432/activity?sslmode=disable

# Development mode (no auth required, dev user treated as admin)
./activity --data-dir ./data --port 8080

# With config file
./activity --config /path/to/config.yaml

# Production (behind auth proxy)
./activity --data-dir /var/lib/activity --port 8080 --host 0.0.0.0
```

Flags: `--port` (default 8080), `--host` (default localhost), `--config`, `--data-dir`, `--debug`, `--version`.

## Web Routes

### Public

- `/` — Dashboard
- `/repos` — Repository list
- `/repos/{name}` — Repository detail
- `/reports/{id}` — Report detail

### Admin (requires authentication)

- `/admin` — Admin dashboard
- `/admin/repos` — Repository management (add, remove, activate, update, analyze)
- `/admin/subscribers` — Newsletter subscriber management
- `/admin/actions` — Trigger analysis and report generation
- `/admin/admins` — Admin user management

## Authentication

Activity is designed to run behind an auth proxy (e.g., OAuth2 Proxy, Authelia). The proxy provides the authenticated user's email in a configurable HTTP header (default: `oidc-email`). Admin access is controlled via the `admins` table in PostgreSQL.

In development mode (`dev_mode: true`), authentication is bypassed and a configurable `dev_user` email is used, automatically treated as admin.

## Configuration

```yaml
data_dir: /var/lib/activity  # Directory for git repository clones

database:
  dsn: postgres://user:pass@localhost:5432/activity?sslmode=disable
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime_seconds: 300

web:
  auth_header: oidc-email       # Header containing user email from auth proxy
  seed_admin: admin@example.com # First admin seeded on empty DB
  dev_mode: false               # Set true for local development
  dev_user: dev@localhost       # Email used in dev mode

llm:
  provider: gemini
  model: gemini-3.0-flash
  api_key_env: GOOGLE_API_KEY
  use_agent: true          # Agent mode (default)
  max_diff_fetches: 5      # Max diffs per analysis
  max_diff_size_kb: 10     # Max size per diff
  max_total_tokens: 100000 # ~$0.01 cost limit

newsletter:
  enabled: true
  sendgrid_api_key_env: SENDGRID_API_KEY

github:
  app_id_env: GITHUB_APP_ID
  installation_id_env: GITHUB_INSTALLATION_ID
  private_key_env: GITHUB_APP_PRIVATE_KEY
```

The database DSN can also be provided via the `DATABASE_URL` environment variable. See `config_example.yaml` for all options including custom prompts.

## How It Works

### Agent Mode (Default)

The intelligent agent:
1. Reviews all commit messages first
2. For **clear messages** (e.g., "Fix null pointer in user auth"): uses message only
3. For **vague messages** (e.g., "fix", "update"): fetches code diff
4. Respects hard limits to prevent cost overruns

**Cost**: ~$0.0005 for well-documented repos, up to ~$0.01 for poorly-documented repos (hard-capped)

### Simple Mode (Fallback)

Sends only commit metadata (SHA, author, date, message) to the LLM. Enable by setting `use_agent: false` in config.

**Cost**: ~$0.0005 per analysis

## Cost Controls

The agent mode includes multiple safeguards:

- **max_diff_fetches**: Limits number of diffs per analysis (default: 5)
- **max_diff_size_kb**: Rejects diffs larger than limit (default: 10KB)
- **max_total_tokens**: Hard cap on total tokens (default: 100K ~ $0.01)
- **Smart prompting**: Agent instructed to use diffs sparingly

## Database

PostgreSQL with migrations managed by [goose](https://github.com/pressly/goose), run automatically on startup.

Tables:
- `repositories`: Tracked repos with metadata
- `activity_runs`: Analysis results with summaries and cost tracking
- `weekly_reports`: Week-indexed summaries keyed by (repo, year, week)
- `subscribers`, `subscriptions`, `newsletter_sends`: Newsletter tables
- `admins`: Admin users for web authentication
- `goose_db_version`: Migration version tracking (managed by goose)

## Development

See `CLAUDE.md` for architecture overview and package descriptions.

### Project Structure

```
main.go               - Entry point, starts web server
internal/
  analyzer/           - Analysis logic (simple + agent modes)
  config/             - Configuration management
  db/                 - PostgreSQL database layer
    migrations/       - Goose SQL migrations (embedded)
  email/              - Email client for newsletters
  forge/              - Forge integration (GitHub, etc.)
  git/                - Git operations
  github/             - GitHub App authentication
  leader/             - Postgres advisory-lock leader election
  llm/                - LLM client abstraction
  newsletter/         - Newsletter composition and sending
  service/            - Business logic layer
  web/                - HTTP server, routes, templates
```

## License

BSD 2-Clause License - see LICENSE file for details
