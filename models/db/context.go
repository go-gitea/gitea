// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"

	"xorm.io/builder"
	"xorm.io/xorm"
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
func (ctx *Context) Value(key any) any {
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
	committer Committer
	committed bool
}

func (c *halfCommitter) Commit() error {
	c.committed = true
	// should do nothing, and the parent committer will commit later
	return nil
}

func (c *halfCommitter) Close() error {
	if c.committed {
		// it's "commit and close", should do nothing, and the parent committer will commit later
		return nil
	}

	// it's "rollback and close", let the parent committer rollback right now
	return c.committer.Close()
}

// TxContext represents a transaction Context,
// it will reuse the existing transaction in the parent context or create a new one.
func TxContext(parentCtx context.Context) (*Context, Committer, error) {
	if sess, ok := inTransaction(parentCtx); ok {
		return newContext(parentCtx, sess, true), &halfCommitter{committer: sess}, nil
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
		err := f(newContext(parentCtx, sess, true))
		if err != nil {
			// rollback immediately, in case the caller ignores returned error and tries to commit the transaction.
			_ = sess.Close()
		}
		return err
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
func Insert(ctx context.Context, beans ...any) error {
	_, err := GetEngine(ctx).Insert(beans...)
	return err
}

// Exec executes a sql with args
func Exec(ctx context.Context, sqlAndArgs ...any) (sql.Result, error) {
	return GetEngine(ctx).Exec(sqlAndArgs...)
}

// GetByBean filled empty fields of the bean according non-empty fields to query in database.
func GetByBean(ctx context.Context, bean any) (bool, error) {
	return GetEngine(ctx).Get(bean)
}

// DeleteByBean deletes all records according non-empty fields of the bean as conditions.
func DeleteByBean(ctx context.Context, bean any) (int64, error) {
	return GetEngine(ctx).Delete(bean)
}

// DeleteByID deletes the given bean with the given ID
func DeleteByID(ctx context.Context, id int64, bean any) (int64, error) {
	return GetEngine(ctx).ID(id).NoAutoTime().Delete(bean)
}

// FindIDs finds the IDs for the given table name satisfying the given condition
// By passing a different value than "id" for "idCol", you can query for foreign IDs, i.e. the repo IDs which satisfy the condition
func FindIDs(ctx context.Context, tableName, idCol string, cond builder.Cond) ([]int64, error) {
	ids := make([]int64, 0, 10)
	if err := GetEngine(ctx).Table(tableName).
		Cols(idCol).
		Where(cond).
		Find(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// DecrByIDs decreases the given column for entities of the "bean" type with one of the given ids by one
// Timestamps of the entities won't be updated
func DecrByIDs(ctx context.Context, ids []int64, decrCol string, bean any) error {
	_, err := GetEngine(ctx).Decr(decrCol).In("id", ids).NoAutoCondition().NoAutoTime().Update(bean)
	return err
}

// DeleteBeans deletes all given beans, beans must contain delete conditions.
func DeleteBeans(ctx context.Context, beans ...any) (err error) {
	e := GetEngine(ctx)
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

// TruncateBeans deletes all given beans, beans may contain delete conditions.
func TruncateBeans(ctx context.Context, beans ...any) (err error) {
	e := GetEngine(ctx)
	for i := range beans {
		if _, err = e.Truncate(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

// CountByBean counts the number of database records according non-empty fields of the bean as conditions.
func CountByBean(ctx context.Context, bean any) (int64, error) {
	return GetEngine(ctx).Count(bean)
}

// TableName returns the table name according a bean object
func TableName(bean any) string {
	return x.TableName(bean)
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
