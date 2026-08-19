package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/teamcutter/go-poker/internal/domain/store"
)

var ErrNotFound = store.ErrNotFound

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func IsConflict(err error) bool { return errors.Is(err, store.ErrConflict) }

func normalize(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return store.ErrConflict
	}
	return err
}
