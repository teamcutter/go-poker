package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teamcutter/go-poker/internal/domain/store"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (d *DB) Close() { d.pool.Close() }

func (d *DB) Raw() *pgxpool.Pool { return d.pool }

func (d *DB) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ctx = store.WithTx(ctx, txWrapper{tx})
	if err := fn(ctx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	Query(ctx context.Context, sql string, args ...any) (store.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) store.Row
}

func (d *DB) q(ctx context.Context) queryer {
	if tx := store.From(ctx); tx != nil {
		return tx
	}
	return dbWrapper{d}
}

func (d *DB) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := d.pool.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

func (d *DB) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rowsWrapper{rows}, nil
}

func (d *DB) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	return d.pool.QueryRow(ctx, sql, args...)
}

type dbWrapper struct{ d *DB }

func (w dbWrapper) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	return w.d.Exec(ctx, sql, args...)
}
func (w dbWrapper) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	return w.d.Query(ctx, sql, args...)
}
func (w dbWrapper) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	return w.d.QueryRow(ctx, sql, args...)
}

type txWrapper struct{ tx pgx.Tx }

func (w txWrapper) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := w.tx.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}
func (w txWrapper) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	rows, err := w.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rowsWrapper{rows}, nil
}
func (w txWrapper) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	return w.tx.QueryRow(ctx, sql, args...)
}

type rowsWrapper struct{ rows pgx.Rows }

func (w rowsWrapper) Next() bool             { return w.rows.Next() }
func (w rowsWrapper) Close() error           { w.rows.Close(); return nil }
func (w rowsWrapper) Err() error             { return w.rows.Err() }
func (w rowsWrapper) Scan(dest ...any) error { return w.rows.Scan(dest...) }
