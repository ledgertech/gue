// Package gopgv10 implements github.com/go-pg/pg/v10 adapter.
package gopgv10

import (
	"context"
	"errors"
	"strings"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"

	"github.com/vgarvardt/gue/v4/adapter"
)

// BUG: using Scan actually executes the query. That’s not an issue given
// how Scan is used in gue.

// BUG: github.com/go-pg/pg/v10 does not accept context in Prepare method so
// as a simple workaround we just convert SQL statements to use placeholders.
// The implementation is good enough for SQL statements used in gue but may
// not work in other cases.

func formatSQL(sql string) string {
	// go-pg uses ? instead of $.
	return strings.Replace(sql, "$", "?", -1)
}

func formatArgs(args []any) []any {
	// go-pg starts at 0 instead of 1.
	dst := make([]any, len(args)+1)
	copy(dst[1:], args)
	return dst
}

// aRow implements adapter.Row using github.com/go-pg/pg/v10
type aRow struct {
	ctx  context.Context
	db   orm.DB
	sql  string
	args []any
}

// Scan implements adapter.Row.Scan() using github.com/go-pg/pg/v10
func (r *aRow) Scan(dest ...any) error {
	_, err := r.db.QueryOneContext(r.ctx, pg.Scan(dest...), formatSQL(r.sql), formatArgs(r.args)...)
	if errors.Is(err, pg.ErrNoRows) {
		return adapter.ErrNoRows
	}
	return err
}

// aCommandTag implements adapter.CommandTag using github.com/go-pg/pg/v10
type aCommandTag struct {
	r orm.Result
}

// RowsAffected implements adapter.CommandTag.RowsAffected() using github.com/go-pg/pg/v10
func (ct aCommandTag) RowsAffected() int64 {
	return int64(ct.r.RowsAffected()) // cast from int
}

// aTx implements adapter.Tx using github.com/go-pg/pg/v10
type aTx struct {
	tx *pg.Tx
}

// NewTx instantiates new adapter.Tx using github.com/go-pg/pg/v10
func NewTx(tx *pg.Tx) adapter.Tx {
	return &aTx{tx: tx}
}

// Exec implements adapter.Tx.Exec() using github.com/go-pg/pg/v10
func (tx *aTx) Exec(ctx context.Context, sql string, args ...any) (adapter.CommandTag, error) {
	r, err := tx.tx.ExecContext(ctx, formatSQL(sql), formatArgs(args)...)
	return aCommandTag{r}, err
}

// QueryRow implements adapter.Tx.QueryRow() using github.com/go-pg/pg/v10
func (tx *aTx) QueryRow(ctx context.Context, sql string, args ...any) adapter.Row {
	return &aRow{ctx, tx.tx, sql, args}
}

// Rollback implements adapter.Tx.Rollback() using github.com/go-pg/pg/v10
func (tx *aTx) Rollback(ctx context.Context) error {
	err := tx.tx.RollbackContext(ctx)
	if errors.Is(err, pg.ErrTxDone) {
		return adapter.ErrTxClosed
	}
	return err
}

// Commit implements adapter.Tx.Commit() using github.com/go-pg/pg/v10
func (tx *aTx) Commit(ctx context.Context) error {
	return tx.tx.CommitContext(ctx)
}

type conn struct {
	c *pg.Conn
}

// NewConn instantiates new adapter.Conn using github.com/go-pg/pg/v10
func NewConn(c *pg.Conn) adapter.Conn {
	return &conn{c}
}

// Ping implements adapter.Conn.Ping() using github.com/go-pg/pg/v10
func (c *conn) Ping(ctx context.Context) error {
	return c.c.Ping(ctx)
}

// Begin implements adapter.Conn.Begin() using github.com/go-pg/pg/v10
func (c *conn) Begin(ctx context.Context) (adapter.Tx, error) {
	tx, err := c.c.BeginContext(ctx)
	return NewTx(tx), err
}

// Exec implements adapter.Conn.Exec() using github.com/go-pg/pg/v10
func (c *conn) Exec(ctx context.Context, sql string, args ...any) (adapter.CommandTag, error) {
	r, err := c.c.ExecContext(ctx, formatSQL(sql), formatArgs(args)...)
	return aCommandTag{r}, err
}

// QueryRow implements adapter.Conn.QueryRow() using github.com/go-pg/pg/v10
func (c *conn) QueryRow(ctx context.Context, sql string, args ...any) adapter.Row {
	return &aRow{ctx, c.c, sql, args}
}

// Release implements adapter.Conn.Release() using github.com/go-pg/pg/v10
func (c *conn) Release() error {
	return c.c.Close()
}

type connPool struct {
	db *pg.DB
}

// NewConnPool instantiates new adapter.ConnPool using github.com/go-pg/pg/v10
func NewConnPool(db *pg.DB) adapter.ConnPool {
	return &connPool{db}
}

// Ping implements adapter.ConnPool.Ping() using github.com/go-pg/pg/v10
func (c *connPool) Ping(ctx context.Context) error {
	return c.db.Ping(ctx)
}

// Begin implements adapter.ConnPool.Begin() using github.com/go-pg/pg/v10
func (c *connPool) Begin(ctx context.Context) (adapter.Tx, error) {
	tx, err := c.db.BeginContext(ctx)
	return NewTx(tx), err
}

// Exec implements adapter.ConnPool.Exec() using github.com/go-pg/pg/v10
func (c *connPool) Exec(ctx context.Context, sql string, args ...any) (adapter.CommandTag, error) {
	r, err := c.db.ExecContext(ctx, formatSQL(sql), formatArgs(args)...)
	return aCommandTag{r}, err
}

// QueryRow implements adapter.ConnPool.QueryRow() using github.com/go-pg/pg/v10
func (c *connPool) QueryRow(ctx context.Context, sql string, args ...any) adapter.Row {
	return &aRow{ctx, c.db, sql, args}
}

// Acquire implements adapter.ConnPool.Acquire() using github.com/go-pg/pg/v10
func (c *connPool) Acquire(_ context.Context) (adapter.Conn, error) {
	return NewConn(c.db.Conn()), nil
}

// Close implements adapter.ConnPool.Close() using github.com/go-pg/pg/v10
func (c *connPool) Close() error {
	return c.db.Close()
}
