package orden

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	c, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configuración de PostgreSQL inválida")
	}
	c.MaxConns = 10
	c.MinConns = 1
	c.MaxConnLifetime = time.Hour
	c.ConnConfig.ConnectTimeout = 10 * time.Second
	db, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("PostgreSQL no disponible")
	}
	return db, nil
}

// A changed environment variable must never silently transfer an activated account.
func ValidateOwner(ctx context.Context, db *pgxpool.Pool, email string) error {
	var matches bool
	if err := db.QueryRow(ctx, "SELECT NOT EXISTS(SELECT 1 FROM users WHERE email<>$1)", email).Scan(&matches); err != nil {
		return fmt.Errorf("no se pudo verificar el propietario")
	}
	if !matches {
		return fmt.Errorf("OWNER_EMAIL no coincide con la cuenta existente; conserva el propietario original")
	}
	return nil
}

func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(728014)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		data, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, string(data)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
