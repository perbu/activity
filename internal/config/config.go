package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	DataDir    string           `yaml:"data_dir"`
	Debug      bool             `yaml:"debug"` // Enable debug logging
	LLM        LLMConfig        `yaml:"llm"`
	Newsletter NewsletterConfig `yaml:"newsletter"`
	GitHub     GitHubConfig     `yaml:"github"`
	Web        WebConfig        `yaml:"web"`
}

// WebConfig represents web server and authentication configuration
type WebConfig struct {
	AuthHeader string `yaml:"auth_header"` // HTTP header containing user email (default: "oidc-email")
	SeedAdmin  string `yaml:"seed_admin"`  // First admin email to create on startup
	DevMode    bool   `yaml:"dev_mode"`    // Bypass auth, use dev_user (for local development)
	DevUser    string `yaml:"dev_user"`    // Email to use in dev mode (default: "dev@localhost")
}

// GitHubConfig represents GitHub App authentication configuration
type GitHubConfig struct {
	AppID             int64  `yaml:"app_id"`
	AppIDEnv          string `yaml:"app_id_env"`           // Env var with App ID
	InstallationID    int64  `yaml:"installation_id"`
	InstallationIDEnv string `yaml:"installation_id_env"`  // Env var with Installation ID
	PrivateKeyPath    string `yaml:"private_key_path"`     // Path to PEM file
	PrivateKeyEnv     string `yaml:"private_key_env"`      // Env var with PEM content
}

// NewsletterConfig represents newsletter email configuration
type NewsletterConfig struct {
	Enabled        bool   `yaml:"enabled"`
	SendGridAPIKey string `yaml:"sendgrid_api_key"`     // Direct API key
	SendGridKeyEnv string `yaml:"sendgrid_api_key_env"` // Environment variable name
	FromEmail      string `yaml:"from_email"`
	FromName       string `yaml:"from_name"`
	SubjectPrefix  string `yaml:"subject_prefix"`
}

// LLMConfig represents LLM provider configuration
type LLMConfig struct {
	Provider         string `yaml:"provider"`
	Model            string `yaml:"model"`
	APIKey           string `yaml:"api_key"`            // Direct API key (takes precedence over api_key_env)
	APIKeyEnv        string `yaml:"api_key_env"`        // Environment variable name containing API key
	MaxCommits       int    `yaml:"max_commits"`        // Max commits to analyze per run
	MaxMessageLength int    `yaml:"max_message_length"` // Max length of commit message to include

	// Phase 3: Agent-based analysis configuration
	UseAgent       bool `yaml:"use_agent"`        // Enable agent-based analysis (default: false)
	MaxDiffFetches int  `yaml:"max_diff_fetches"` // Max diffs agent can fetch per analysis (default: 5)
	MaxDiffSizeKB  int  `yaml:"max_diff_size_kb"` // Max size of each diff in KB (default: 10)
	MaxTotalTokens int  `yaml:"max_total_tokens"` // Max total tokens for agent session (default: 100000)
	EnableToolLogs bool `yaml:"enable_tool_logs"` // Enable detailed tool execution logs (default: true)

	// Prompt customization (optional overrides)
	Phase2Prompt      string `yaml:"phase2_prompt"`       // Custom prompt for Phase 2 simple LLM analysis
	AgentSystemPrompt string `yaml:"agent_system_prompt"` // Custom system instruction for Phase 3 agent
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DataDir: "", // Must be specified by user
		LLM: LLMConfig{
			Provider:         "gemini",
			Model:            "gemini-3.0-flash",
			APIKeyEnv:        "GOOGLE_API_KEY",
			MaxCommits:       50,   // Limit to 50 commits per analysis
			MaxMessageLength: 1000, // Truncate long commit messages

			// Phase 3: Agent mode (default) - intelligent diff fetching
			UseAgent:       true,   // Agent mode by default (set false for Phase 2)
			MaxDiffFetches: 5,      // Max 5 diffs per analysis
			MaxDiffSizeKB:  10,     // Max 10KB per diff
			MaxTotalTokens: 100000, // ~$0.01 cost limit
			EnableToolLogs: true,   // Enable logging for debugging
		},
		Newsletter: NewsletterConfig{
			Enabled:        false,
			SendGridKeyEnv: "SENDGRID_API_KEY",
			FromEmail:      "activity@example.com",
			FromName:       "Activity Digest",
			SubjectPrefix:  "[Activity]",
		},
		GitHub: GitHubConfig{
			AppIDEnv:          "GITHUB_APP_ID",
			InstallationIDEnv: "GITHUB_INSTALLATION_ID",
			PrivateKeyEnv:     "GITHUB_APP_PRIVATE_KEY",
		},
		Web: WebConfig{
			AuthHeader: "oidc-email",
			DevUser:    "dev@localhost",
		},
	}
}

// GetSeedAdmin returns the seed admin email from config or environment
func (c *Config) GetSeedAdmin() string {
	if c.Web.SeedAdmin != "" {
		return c.Web.SeedAdmin
	}
	return os.Getenv("ACTIVITY_SEED_ADMIN")
}

// GetAuthHeader returns the configured auth header name
func (c *Config) GetAuthHeader() string {
	if c.Web.AuthHeader != "" {
		return c.Web.AuthHeader
	}
	return "oidc-email"
}

// GetDevUser returns the dev mode user email
func (c *Config) GetDevUser() string {
	if c.Web.DevUser != "" {
		return c.Web.DevUser
	}
	return "dev@localhost"
}

// Load loads configuration from the specified path, falling back to defaults
func Load(configPath string) (*Config, error) {
	// If no path specified, use default location
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".config", "activity", "config.yaml")
	}

	// Expand ~ in path
	configPath = expandPath(configPath)

	// Start with defaults
	cfg := DefaultConfig()

	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, return defaults
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand ~ in data_dir if present
	cfg.DataDir = expandPath(cfg.DataDir)

	return cfg, nil
}

// expandPath expands ~ to home directory in paths
func expandPath(path string) string {
	if path == "" {
		return path
	}

	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if len(path) == 1 {
			return homeDir
		}
		return filepath.Join(homeDir, path[1:])
	}

	return path
}

// EnsureDataDir creates the data directory if it doesn't exist
func (c *Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	return nil
}

// GetPhase2Prompt returns the Phase 2 prompt, either custom or default
func (c *Config) GetPhase2Prompt() string {
	if c.LLM.Phase2Prompt != "" {
		return c.LLM.Phase2Prompt
	}
	return DefaultPhase2Prompt
}

// GetAgentSystemPrompt returns the agent system prompt, either custom or default
func (c *Config) GetAgentSystemPrompt() string {
	if c.LLM.AgentSystemPrompt != "" {
		return c.LLM.AgentSystemPrompt
	}
	return DefaultAgentSystemPrompt
}

// DefaultPhase2Prompt is the default prompt template for Phase 2 analysis
const DefaultPhase2Prompt = `Please provide a concise summary of the development activity in this commit range.
Focus on:
1. Main features or changes implemented
2. Bug fixes
3. Refactoring or code improvements
4. Notable patterns or trends

Keep the summary under 300 words and use clear, professional language.`

// GetSendGridAPIKey returns the SendGrid API key, checking direct key first then env var
func (c *Config) GetSendGridAPIKey() string {
	if c.Newsletter.SendGridAPIKey != "" {
		return c.Newsletter.SendGridAPIKey
	}
	if c.Newsletter.SendGridKeyEnv != "" {
		return os.Getenv(c.Newsletter.SendGridKeyEnv)
	}
	return ""
}

// HasGitHubApp returns true if GitHub App authentication is configured
func (c *Config) HasGitHubApp() bool {
	return c.GetGitHubAppID() != 0 && c.GetGitHubInstallationID() != 0
}

// GetGitHubAppID returns the GitHub App ID, checking direct value first then env var
func (c *Config) GetGitHubAppID() int64 {
	if c.GitHub.AppID != 0 {
		return c.GitHub.AppID
	}
	if c.GitHub.AppIDEnv != "" {
		if val := os.Getenv(c.GitHub.AppIDEnv); val != "" {
			if id, err := strconv.ParseInt(val, 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

// GetGitHubInstallationID returns the GitHub Installation ID, checking direct value first then env var
func (c *Config) GetGitHubInstallationID() int64 {
	if c.GitHub.InstallationID != 0 {
		return c.GitHub.InstallationID
	}
	if c.GitHub.InstallationIDEnv != "" {
		if val := os.Getenv(c.GitHub.InstallationIDEnv); val != "" {
			if id, err := strconv.ParseInt(val, 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

// GetGitHubPrivateKey returns the GitHub App private key, checking file path first then env var
func (c *Config) GetGitHubPrivateKey() ([]byte, error) {
	// Check file path first
	if c.GitHub.PrivateKeyPath != "" {
		path := expandPath(c.GitHub.PrivateKeyPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		return data, nil
	}

	// Check environment variable
	if c.GitHub.PrivateKeyEnv != "" {
		key := os.Getenv(c.GitHub.PrivateKeyEnv)
		if key != "" {
			return []byte(key), nil
		}
	}

	return nil, fmt.Errorf("no GitHub App private key configured")
}

// DefaultAgentSystemPrompt is the default system instruction for Phase 3 agent
const DefaultAgentSystemPrompt = `You are a Git commit analyzer that summarizes development activity.

Your goal is to produce a concise summary of what happened in this commit range.

IMPORTANT GUIDELINES:
1. First, review all commit messages provided in the user prompt
2. If a commit message is CLEAR and DESCRIPTIVE (e.g., "Fix null pointer in user auth",
   "Add pagination to API endpoint"), you can summarize it WITHOUT viewing the diff
3. ONLY use get_commit_diff when:
   - The commit message is vague (e.g., "fix", "update", "changes", "stuff")
   - The message doesn't explain WHAT was changed
   - You need to verify the scope of a change
   - The message references a ticket/issue without explanation (e.g., "Fix #123")
4. You have LIMITED diff fetches (max %d per analysis) - use them wisely
5. Before fetching a diff, consider using get_full_commit_message if the message was truncated
6. Prioritize diffs for:
   - Unclear messages that seem important
   - Commits that likely have significant impact
   - Bug fixes without clear descriptions
7. Use get_author_stats to get information about contributors when there are multiple
   authors or when you want to provide context about who is contributing

OUTPUT FORMAT:
Provide a summary with these sections:
1. Main Features or Changes: New capabilities added
2. Bug Fixes: Issues resolved
3. Refactoring/Improvements: Code quality changes
4. Notable Patterns: Trends across commits (if any)
5. Contributors: Brief info about active authors (use get_author_stats for context)

Keep the summary under 400 words and use clear, professional language.
If you had to skip analyzing some commits due to limits, mention this briefly at the end.`

// DefaultDescriptionPrompt is the prompt used to generate repository descriptions from README files
const DefaultDescriptionPrompt = `Summarize this software project in 2-3 sentences for someone who will be reading commit summaries. Focus on:
- What the project IS (tool, library, service, etc.)
- What problem it solves or what it's used for
- Key technical domain (if relevant)

Do NOT include:
- Installation instructions
- File structure details
- Version numbers
- Contributor information

README content:
---
%s
---

Provide only the summary, no preamble.`
