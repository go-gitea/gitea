// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// DefaultContext is the default context to run xorm queries in
// will be overwritten by Init with HammerContext
var DefaultContext context.Context

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// enginedContextKey is a context key. It is used with context.Value() to get the current Engined for the context
var (
	enginedContextKey         = &contextKey{"engined"}
	_                 Engined = &Context{}
)

// Context represents a db context
type Context struct {
	context.Context
	e           Engine
	transaction bool
}

func newContext(ctx context.Context, e Engine, transaction bool) *Context {
	return &Context{
		Context:     ctx,
		e:           e,
		transaction: transaction,
	}
}

// InTransaction if context is in a transaction
func (ctx *Context) InTransaction() bool {
	return ctx.transaction
}

// Engine returns db engine
func (ctx *Context) Engine() Engine {
	return ctx.e
}

// Value shadows Value for context.Context but allows us to get ourselves and an Engined object
func (ctx *Context) Value(key interface{}) interface{} {
	if key == enginedContextKey {
		return ctx
	}
	return ctx.Context.Value(key)
}

// WithContext returns this engine tied to this context
func (ctx *Context) WithContext(other context.Context) *Context {
	return newContext(ctx, ctx.e.Context(other), ctx.transaction)
}

// Engined structs provide an Engine
type Engined interface {
	Engine() Engine
}

// GetEngine will get a db Engine from this context or return an Engine restricted to this context
func GetEngine(ctx context.Context) Engine {
	if e := getEngine(ctx); e != nil {
		return e
	}
	return x.Context(ctx)
}

// getEngine will get a db Engine from this context or return nil
func getEngine(ctx context.Context) Engine {
	if engined, ok := ctx.(Engined); ok {
		return engined.Engine()
	}
	enginedInterface := ctx.Value(enginedContextKey)
	if enginedInterface != nil {
		return enginedInterface.(Engined).Engine()
	}
	return nil
}

// Committer represents an interface to Commit or Close the Context
type Committer interface {
	Commit() error
	Close() error
}

// halfCommitter is a wrapper of Committer.
// It can be closed early, but can't be committed early, it is useful for reusing a transaction.
type halfCommitter struct {
	Committer
}

func (*halfCommitter) Commit() error {
	// do nothing
	return nil
}

// TxContext represents a transaction Context,
// it will reuse the existing transaction in the parent context or create a new one.
func TxContext(parentCtx context.Context) (*Context, Committer, error) {
	if sess, ok := inTransaction(parentCtx); ok {
		return newContext(parentCtx, sess, true), &halfCommitter{Committer: sess}, nil
	}

	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		sess.Close()
		return nil, nil, err
	}

	return newContext(DefaultContext, sess, true), sess, nil
}

// WithTx represents executing database operations on a transaction, if the transaction exist,
// this function will reuse it otherwise will create a new one and close it when finished.
func WithTx(parentCtx context.Context, f func(ctx context.Context) error) error {
	if sess, ok := inTransaction(parentCtx); ok {
		return f(newContext(parentCtx, sess, true))
	}
	return txWithNoCheck(parentCtx, f)
}

func txWithNoCheck(parentCtx context.Context, f func(ctx context.Context) error) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := f(newContext(parentCtx, sess, true)); err != nil {
		return err
	}

	return sess.Commit()
}

// Insert inserts records into database
func Insert(ctx context.Context, beans ...interface{}) error {
	_, err := GetEngine(ctx).Insert(beans...)
	return err
}

// Exec executes a sql with args
func Exec(ctx context.Context, sqlAndArgs ...interface{}) (sql.Result, error) {
	return GetEngine(ctx).Exec(sqlAndArgs...)
}

// GetByBean filled empty fields of the bean according non-empty fields to query in database.
func GetByBean(ctx context.Context, bean interface{}) (bool, error) {
	return GetEngine(ctx).Get(bean)
}

// DeleteByBean deletes all records according non-empty fields of the bean as conditions.
func DeleteByBean(ctx context.Context, bean interface{}) (int64, error) {
	return GetEngine(ctx).Delete(bean)
}

// DeleteBeans deletes all given beans, beans should contain delete conditions.
func DeleteBeans(ctx context.Context, beans ...interface{}) (err error) {
	e := GetEngine(ctx)
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

// CountByBean counts the number of database records according non-empty fields of the bean as conditions.
func CountByBean(ctx context.Context, bean interface{}) (int64, error) {
	return GetEngine(ctx).Count(bean)
}

// TableName returns the table name according a bean object
func TableName(bean interface{}) string {
	return x.TableName(bean)
}

// EstimateCount returns an estimate of total number of rows in table
func EstimateCount(ctx context.Context, bean interface{}) (int64, error) {
	e := GetEngine(ctx)
	e.Context(ctx)

	var rows int64
	var err error
	tablename := TableName(bean)
	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err = e.Context(ctx).SQL("SELECT table_rows FROM information_schema.tables WHERE tables.table_name = ? AND tables.table_schema = ?;", tablename, x.Dialect().URI().DBName).Get(&rows)
	case schemas.POSTGRES:
		// the table can live in multiple schemas of a postgres database
		// See https://wiki.postgresql.org/wiki/Count_estimate
		tablename = x.TableName(bean, true)
		_, err = e.Context(ctx).SQL("SELECT reltuples::bigint AS estimate FROM pg_class WHERE oid = ?::regclass;", tablename).Get(&rows)
	case schemas.MSSQL:
		_, err = e.Context(ctx).SQL("sp_spaceused ?;", tablename).Get(&rows)
	default:
		return e.Context(ctx).Count(tablename)
	}
	return rows, err
}

// InTransaction returns true if the engine is in a transaction otherwise return false
func InTransaction(ctx context.Context) bool {
	_, ok := inTransaction(ctx)
	return ok
}

func inTransaction(ctx context.Context) (*xorm.Session, bool) {
	e := getEngine(ctx)
	if e == nil {
		return nil, false
	}

	switch t := e.(type) {
	case *xorm.Engine:
		return nil, false
	case *xorm.Session:
		if t.IsInTx() {
			return t, true
		}
		return nil, false
	default:
		return nil, false
	}
}
