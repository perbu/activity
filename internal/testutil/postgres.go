// Package testutil provides shared helpers for integration tests.
//
// It is intended to be imported only from _test.go files. Doing so keeps the
// testcontainers-go dependency out of the production binary, even though Go's
// go.mod does not formally distinguish test-only dependencies.
package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartPostgresContainer launches a Postgres 16-alpine testcontainer and
// returns its DSN (with sslmode=disable). Callers must invoke terminate to
// shut the container down — typically via defer in TestMain.
func StartPostgresContainer(ctx context.Context) (dsn string, terminate func(), err error) {
	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
				wait.ForListeningPort("5432/tcp"),
			),
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pg.Terminate(ctx)
		return "", nil, fmt.Errorf("get connection string: %w", err)
	}

	return connStr, func() { pg.Terminate(ctx) }, nil
}
