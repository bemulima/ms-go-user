// Command user-service-migrate applies the repository SQL migrations through DB_DSN.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationTable = "schema_migrations"

type state struct {
	database      string
	role          string
	superuser     bool
	historyExists bool
	tableCount    int
	version       uint
	dirty         bool
}

func main() {
	action := "status"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	if action != "status" && action != "up" && action != "down" {
		log.Fatalf("usage: user-service-migrate [status|up|down]")
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect PostgreSQL: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping PostgreSQL: %v", err)
	}

	current, err := inspect(ctx, pool)
	if err != nil {
		log.Fatalf("inspect migration state: %v", err)
	}
	if action == "status" {
		printStatus(current)
		return
	}
	if !current.historyExists && current.tableCount > 0 {
		log.Fatalf("refusing to migrate an untracked schema with %d existing public table(s); inspect and establish migration history manually", current.tableCount)
	}

	migrations, err := openMigrations(dsn)
	if err != nil {
		log.Fatalf("open migrations: %v", err)
	}
	defer migrations.Close()

	switch action {
	case "up":
		err = migrations.Up()
	case "down":
		err = migrations.Down()
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("%s migrations: %v", action, err)
	}

	current, err = inspect(ctx, pool)
	if err != nil {
		log.Fatalf("inspect migration state after %s: %v", action, err)
	}
	printStatus(current)
}

func openMigrations(dsn string) (*migrate.Migrate, error) {
	directory, err := filepath.Abs("migrations")
	if err != nil {
		return nil, err
	}
	return migrate.New("file://"+filepath.ToSlash(directory), dsn)
}

func inspect(ctx context.Context, pool *pgxpool.Pool) (state, error) {
	current := state{}
	if err := pool.QueryRow(ctx, "SELECT current_database(), current_user, r.rolsuper FROM pg_roles r WHERE r.rolname = current_user").Scan(&current.database, &current.role, &current.superuser); err != nil {
		return state{}, err
	}
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public."+migrationTable+"') IS NOT NULL").Scan(&current.historyExists); err != nil {
		return state{}, err
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename <> $1", migrationTable).Scan(&current.tableCount); err != nil {
		return state{}, err
	}
	if current.historyExists {
		if err := pool.QueryRow(ctx, "SELECT version, dirty FROM "+migrationTable+" LIMIT 1").Scan(&current.version, &current.dirty); err != nil {
			return state{}, err
		}
	}
	return current, nil
}

func printStatus(current state) {
	history := "absent"
	if current.historyExists {
		history = "present"
	}
	fmt.Printf("database=%s role=%s superuser=%t public_tables=%d migration_history=%s version=%d dirty=%t\n", current.database, current.role, current.superuser, current.tableCount, history, current.version, current.dirty)
}
