package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used across every repository to map a
// duplicate insert onto the owning domain package's ErrAlreadyExists.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
