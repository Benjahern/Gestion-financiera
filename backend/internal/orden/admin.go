package orden

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// IssueCode is intentionally available only to the server-side CLI, never HTTP.
func IssueCode(ctx context.Context, db *pgxpool.Pool, purpose string) (string, error) {
	if purpose != "activate" && purpose != "recover" {
		return "", errors.New("propósito inválido")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(728015)"); err != nil {
		return "", err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=1)").Scan(&exists); err != nil {
		return "", err
	}
	if (purpose == "activate" && exists) || (purpose == "recover" && !exists) {
		return "", errors.New("estado de la cuenta incompatible con el comando")
	}
	code, err := randomToken()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO activation_codes(purpose,token_hash,expires_at) VALUES($1,$2,now()+interval '15 minutes') ON CONFLICT(purpose) DO UPDATE SET token_hash=excluded.token_hash,expires_at=excluded.expires_at", purpose, tokenHash(code))
	if err != nil {
		return "", err
	}
	if purpose == "recover" {
		if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=1"); err != nil {
			return "", err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return code, nil
}
