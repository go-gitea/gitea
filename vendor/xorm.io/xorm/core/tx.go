// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"context"
	"database/sql"

	"xorm.io/xorm/contexts"
)

var (
	_ QueryExecuter = &Tx{}
)

// Tx represents a transaction
type Tx struct {
	*sql.Tx
	db  *DB
	ctx context.Context
}

// BeginTx begin a transaction with option
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	hookCtx := contexts.NewContextHook(ctx, "BEGIN TRANSACTION", nil)
	ctx, err := db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	tx, err := db.DB.BeginTx(ctx, opts)
	hookCtx.End(ctx, nil, err)
	if err := db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return &Tx{tx, db, ctx}, nil
}

// Begin begins a transaction
func (db *DB) Begin() (*Tx, error) {
	return db.BeginTx(context.Background(), nil)
}

// Commit submit the transaction
func (tx *Tx) Commit() error {
	hookCtx := contexts.NewContextHook(tx.ctx, "COMMIT", nil)
	ctx, err := tx.db.beforeProcess(hookCtx)
	if err != nil {
		return err
	}
	err = tx.Tx.Commit()
	hookCtx.End(ctx, nil, err)
	return tx.db.afterProcess(hookCtx)
}

// Rollback rollback the transaction
func (tx *Tx) Rollback() error {
	hookCtx := contexts.NewContextHook(tx.ctx, "ROLLBACK", nil)
	ctx, err := tx.db.beforeProcess(hookCtx)
	if err != nil {
		return err
	}
	err = tx.Tx.Rollback()
	hookCtx.End(ctx, nil, err)
	return tx.db.afterProcess(hookCtx)
}

// PrepareContext prepare the query
func (tx *Tx) PrepareContext(ctx context.Context, query string) (*Stmt, error) {
	names := make(map[string]int)
	var i int
	query = re.ReplaceAllStringFunc(query, func(src string) string {
		names[src[1:]] = i
		i++
		return "?"
	})
	hookCtx := contexts.NewContextHook(ctx, "PREPARE", nil)
	ctx, err := tx.db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	stmt, err := tx.Tx.PrepareContext(ctx, query)
	hookCtx.End(ctx, nil, err)
	if err := tx.db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return &Stmt{stmt, tx.db, names, query}, nil
}

// Prepare prepare the query
func (tx *Tx) Prepare(query string) (*Stmt, error) {
	return tx.PrepareContext(context.Background(), query)
}

// StmtContext creates Stmt with context
func (tx *Tx) StmtContext(ctx context.Context, stmt *Stmt) *Stmt {
	stmt.Stmt = tx.Tx.StmtContext(ctx, stmt.Stmt)
	return stmt
}

// Stmt creates Stmt
func (tx *Tx) Stmt(stmt *Stmt) *Stmt {
	return tx.StmtContext(context.Background(), stmt)
}

// ExecMapContext executes query with args in a map
func (tx *Tx) ExecMapContext(ctx context.Context, query string, mp interface{}) (sql.Result, error) {
	query, args, err := MapToSlice(query, mp)
	if err != nil {
		return nil, err
	}
	return tx.ExecContext(ctx, query, args...)
}

// ExecMap executes query with args in a map
func (tx *Tx) ExecMap(query string, mp interface{}) (sql.Result, error) {
	return tx.ExecMapContext(context.Background(), query, mp)
}

// ExecStructContext executes query with args in a struct
func (tx *Tx) ExecStructContext(ctx context.Context, query string, st interface{}) (sql.Result, error) {
	query, args, err := StructToSlice(query, st)
	if err != nil {
		return nil, err
	}
	return tx.ExecContext(ctx, query, args...)
}

// ExecContext executes a query with args
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	hookCtx := contexts.NewContextHook(ctx, query, args)
	ctx, err := tx.db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	res, err := tx.Tx.ExecContext(ctx, query, args...)
	hookCtx.End(ctx, res, err)
	if err := tx.db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return res, err
}

// ExecStruct executes query with args in a struct
func (tx *Tx) ExecStruct(query string, st interface{}) (sql.Result, error) {
	return tx.ExecStructContext(context.Background(), query, st)
}

// QueryContext query with args
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	hookCtx := contexts.NewContextHook(ctx, query, args)
	ctx, err := tx.db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Tx.QueryContext(ctx, query, args...)
	hookCtx.End(ctx, nil, err)
	if err := tx.db.afterProcess(hookCtx); err != nil {
		if rows != nil {
			rows.Close()
		}
		return nil, err
	}
	return &Rows{rows, tx.db}, nil
}

// Query query with args
func (tx *Tx) Query(query string, args ...interface{}) (*Rows, error) {
	return tx.QueryContext(context.Background(), query, args...)
}

// QueryMapContext query with args in a map
func (tx *Tx) QueryMapContext(ctx context.Context, query string, mp interface{}) (*Rows, error) {
	query, args, err := MapToSlice(query, mp)
	if err != nil {
		return nil, err
	}
	return tx.QueryContext(ctx, query, args...)
}

// QueryMap query with args in a map
func (tx *Tx) QueryMap(query string, mp interface{}) (*Rows, error) {
	return tx.QueryMapContext(context.Background(), query, mp)
}

// QueryStructContext query with args in struct
func (tx *Tx) QueryStructContext(ctx context.Context, query string, st interface{}) (*Rows, error) {
	query, args, err := StructToSlice(query, st)
	if err != nil {
		return nil, err
	}
	return tx.QueryContext(ctx, query, args...)
}

// QueryStruct query with args in struct
func (tx *Tx) QueryStruct(query string, st interface{}) (*Rows, error) {
	return tx.QueryStructContext(context.Background(), query, st)
}

// QueryRowContext query one row with args
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *Row {
	rows, err := tx.QueryContext(ctx, query, args...)
	return &Row{rows, err}
}

// QueryRow query one row with args
func (tx *Tx) QueryRow(query string, args ...interface{}) *Row {
	return tx.QueryRowContext(context.Background(), query, args...)
}

// QueryRowMapContext query one row with args in a map
func (tx *Tx) QueryRowMapContext(ctx context.Context, query string, mp interface{}) *Row {
	query, args, err := MapToSlice(query, mp)
	if err != nil {
		return &Row{nil, err}
	}
	return tx.QueryRowContext(ctx, query, args...)
}

// QueryRowMap query one row with args in a map
func (tx *Tx) QueryRowMap(query string, mp interface{}) *Row {
	return tx.QueryRowMapContext(context.Background(), query, mp)
}

// QueryRowStructContext query one row with args in struct
func (tx *Tx) QueryRowStructContext(ctx context.Context, query string, st interface{}) *Row {
	query, args, err := StructToSlice(query, st)
	if err != nil {
		return &Row{nil, err}
	}
	return tx.QueryRowContext(ctx, query, args...)
}

// QueryRowStruct query one row with args in struct
func (tx *Tx) QueryRowStruct(query string, st interface{}) *Row {
	return tx.QueryRowStructContext(context.Background(), query, st)
}
