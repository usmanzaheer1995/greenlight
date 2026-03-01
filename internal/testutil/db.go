package testutil

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPostgres(migrate func(*sql.DB) error) (*sql.DB, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_USER":     "test",
			"POSTGRES_DB":       "greenlight_test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		panic(fmt.Sprintf("could not start postgres container: %v", err))
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		pgContainer.Terminate(ctx)
		panic(fmt.Sprintf("could not get mapped port: %v", err))
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		pgContainer.Terminate(ctx)
		panic(fmt.Sprintf("could not get container host: %v", err))
	}

	dsn := fmt.Sprintf(
		"postgres://test:test@%s:%s/greenlight_test?sslmode=disable",
		host, mappedPort.Port(),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		pgContainer.Terminate(ctx)
		panic(fmt.Sprintf("could not open test database: %v", err))
	}

	cleanup := func() {
		db.Close()
		pgContainer.Terminate(ctx)
	}

	if err := migrate(db); err != nil {
		pgContainer.Terminate(ctx)
		panic(fmt.Sprintf("could not apply migrations: %v", err))
	}

	return db, cleanup
}
