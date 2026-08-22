package goql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// OnFailure defines transaction behavior when a scoped function returns an error.
type OnFailure int

const (
	// OnFailureRollback rolls back the transaction on error (default).
	OnFailureRollback OnFailure = iota
	// OnFailureCommit commits the transaction even when fn returns an error.
	OnFailureCommit
	// OnFailurePropagate returns the error without rolling back (caller owns the tx).
	OnFailurePropagate
)

// TxOptions configures scoped transaction behavior.
type TxOptions struct {
	OnFailure  OnFailure
	Isolation  sql.IsolationLevel
	ReadOnly   bool
	TxOptions  *sql.TxOptions
}

// DefaultTxOptions returns rollback-on-error defaults.
func DefaultTxOptions() TxOptions {
	return TxOptions{OnFailure: OnFailureRollback}
}

type txKey struct{}

// Tx runs fn inside a transaction. Nested Tx calls reuse the outer transaction
// and produce a single commit when the outermost scope succeeds.
func (db *DB) Tx(ctx context.Context, fn func(ctx context.Context, tx *DB) error) error {
	return db.TxWith(ctx, DefaultTxOptions(), fn)
}

// TxWith runs fn inside a transaction with explicit failure handling.
func (db *DB) TxWith(ctx context.Context, opts TxOptions, fn func(ctx context.Context, tx *DB) error) error {
	if existing := txFromContext(ctx); existing != nil {
		return fn(ctx, existing)
	}

	txOpts := opts.TxOptions
	if txOpts == nil && (opts.Isolation != 0 || opts.ReadOnly) {
		txOpts = &sql.TxOptions{
			Isolation: opts.Isolation,
			ReadOnly:  opts.ReadOnly,
		}
	}

	tx, err := db.BeginTxx(ctx, txOpts)
	if err != nil {
		return fmt.Errorf("ameego: begin tx: %w", err)
	}

	txDB := db.WithTx(tx)
	txDB.inTx = true
	ctx = context.WithValue(ctx, txKey{}, txDB)

	if err := fn(ctx, txDB); err != nil {
		switch opts.OnFailure {
		case OnFailureCommit:
			if commitErr := tx.Commit(); commitErr != nil {
				return fmt.Errorf("ameego: commit after error: %w (original: %v)", commitErr, err)
			}
			return err
		case OnFailurePropagate:
			return err
		default:
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				return fmt.Errorf("ameego: rollback: %w (original: %v)", rbErr, err)
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ameego: commit: %w", err)
	}
	return nil
}

// Session runs fn with a dedicated connection from the pool.
// Useful for SET statements or session-scoped configuration without a transaction.
func (db *DB) Session(ctx context.Context, fn func(ctx context.Context, conn *DB) error) error {
	type connBegin interface {
		Connx(ctx context.Context) (*sqlx.Conn, error)
	}
	cb, ok := db.raw().(connBegin)
	if !ok {
		return fn(ctx, db)
	}
	c, err := cb.Connx(ctx)
	if err != nil {
		return fmt.Errorf("ameego: session conn: %w", err)
	}
	defer c.Close() //nolint:errcheck

	sessionDB := &DB{inner: c, dialect: db.dialect, inTx: false}
	return fn(ctx, sessionDB)
}

// InTx reports whether this DB handle is bound to an active transaction.
func (db *DB) InTx() bool {
	return db.inTx
}

func txFromContext(ctx context.Context) *DB {
	v, _ := ctx.Value(txKey{}).(*DB)
	return v
}

func (db *DB) raw() sqlxQueryer {
	return db.inner
}
