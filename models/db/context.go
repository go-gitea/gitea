// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"
	"errors"
	"runtime"
	"slices"
	"sync"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// DefaultContext is the default context to run xorm queries in
// will be overwritten by Init with HammerContext
var DefaultContext context.Context

type engineContextKeyType struct{}

var engineContextKey = engineContextKeyType{}

// Context represents a db context
type Context struct {
	context.Context
	engine Engine
}

func newContext(ctx context.Context, e Engine) *Context {
	return &Context{Context: ctx, engine: e}
}

// Value shadows Value for context.Context but allows us to get ourselves and an Engined object
func (ctx *Context) Value(key any) any {
	if key == engineContextKey {
		return ctx
	}
	return ctx.Context.Value(key)
}

// WithContext returns this engine tied to this context
func (ctx *Context) WithContext(other context.Context) *Context {
	return newContext(ctx, ctx.engine.Context(other))
}

var (
	contextSafetyOnce          sync.Once
	contextSafetyDeniedFuncPCs []uintptr
)

func contextSafetyCheck(e Engine) {
	if setting.IsProd && !setting.IsInTesting {
		return
	}
	if e == nil {
		return
	}
	// Only do this check for non-end-users. If the problem could be fixed in the future, this code could be removed.
	contextSafetyOnce.Do(func() {
		// try to figure out the bad functions to deny
		type m struct{}
		_ = e.SQL("SELECT 1").Iterate(&m{}, func(int, any) error {
			callers := make([]uintptr, 32)
			callerNum := runtime.Callers(1, callers)
			for i := 0; i < callerNum; i++ {
				if funcName := runtime.FuncForPC(callers[i]).Name(); funcName == "xorm.io/xorm.(*Session).Iterate" {
					contextSafetyDeniedFuncPCs = append(contextSafetyDeniedFuncPCs, callers[i])
				}
			}
			return nil
		})
		if len(contextSafetyDeniedFuncPCs) != 1 {
			panic(errors.New("unable to determine the functions to deny"))
		}
	})

	// it should be very fast: xxxx ns/op
	callers := make([]uintptr, 32)
	callerNum := runtime.Callers(3, callers) // skip 3: runtime.Callers, contextSafetyCheck, GetEngine
	for i := 0; i < callerNum; i++ {
		if slices.Contains(contextSafetyDeniedFuncPCs, callers[i]) {
			panic(errors.New("using database context in an iterator would cause corrupted results"))
		}
	}
}

// GetEngine gets an existing db Engine/Statement or creates a new Session
func GetEngine(ctx context.Context) Engine {
	if e := getExistingEngine(ctx); e != nil {
		return e
	}
	return xormEngine.Context(ctx)
}

// getExistingEngine gets an existing db Engine/Statement from this context or returns nil
func getExistingEngine(ctx context.Context) (e Engine) {
	defer func() { contextSafetyCheck(e) }()
	if engined, ok := ctx.(*Context); ok {
		return engined.engine
	}
	if engined, ok := ctx.Value(engineContextKey).(*Context); ok {
		return engined.engine
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
// Some tips to use:
//
//	1 It's always recommended to use `WithTx` in new code instead of `TxContext`, since `WithTx` will handle the transaction automatically.
//	2. To maintain the old code which uses `TxContext`:
//	  a. Always call `Close()` before returning regardless of whether `Commit()` has been called.
//	  b. Always call `Commit()` before returning if there are no errors, even if the code did not change any data.
//	  c. Remember the `Committer` will be a halfCommitter when a transaction is being reused.
//	     So calling `Commit()` will do nothing, but calling `Close()` without calling `Commit()` will rollback the transaction.
//	     And all operations submitted by the caller stack will be rollbacked as well, not only the operations in the current function.
//	  d. It doesn't mean rollback is forbidden, but always do it only when there is an error, and you do want to rollback.
func TxContext(parentCtx context.Context) (*Context, Committer, error) {
	if sess, ok := inTransaction(parentCtx); ok {
		return newContext(parentCtx, sess), &halfCommitter{committer: sess}, nil
	}

	sess := xormEngine.NewSession()
	if err := sess.Begin(); err != nil {
		_ = sess.Close()
		return nil, nil, err
	}

	return newContext(DefaultContext, sess), sess, nil
}

// WithTx represents executing database operations on a transaction, if the transaction exist,
// this function will reuse it otherwise will create a new one and close it when finished.
func WithTx(parentCtx context.Context, f func(ctx context.Context) error) error {
	if sess, ok := inTransaction(parentCtx); ok {
		err := f(newContext(parentCtx, sess))
		if err != nil {
			// rollback immediately, in case the caller ignores returned error and tries to commit the transaction.
			_ = sess.Close()
		}
		return err
	}
	return txWithNoCheck(parentCtx, f)
}

func txWithNoCheck(parentCtx context.Context, f func(ctx context.Context) error) error {
	sess := xormEngine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := f(newContext(parentCtx, sess)); err != nil {
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

func Get[T any](ctx context.Context, cond builder.Cond) (object *T, exist bool, err error) {
	if !cond.IsValid() {
		panic("cond is invalid in db.Get(ctx, cond). This should not be possible.")
	}

	var bean T
	has, err := GetEngine(ctx).Where(cond).NoAutoCondition().Get(&bean)
	if err != nil {
		return nil, false, err
	} else if !has {
		return nil, false, nil
	}
	return &bean, true, nil
}

func GetByID[T any](ctx context.Context, id int64) (object *T, exist bool, err error) {
	var bean T
	has, err := GetEngine(ctx).ID(id).NoAutoCondition().Get(&bean)
	if err != nil {
		return nil, false, err
	} else if !has {
		return nil, false, nil
	}
	return &bean, true, nil
}

func Exist[T any](ctx context.Context, cond builder.Cond) (bool, error) {
	if !cond.IsValid() {
		panic("cond is invalid in db.Exist(ctx, cond). This should not be possible.")
	}

	var bean T
	return GetEngine(ctx).Where(cond).NoAutoCondition().Exist(&bean)
}

func ExistByID[T any](ctx context.Context, id int64) (bool, error) {
	var bean T
	return GetEngine(ctx).ID(id).NoAutoCondition().Exist(&bean)
}

// DeleteByID deletes the given bean with the given ID
func DeleteByID[T any](ctx context.Context, id int64) (int64, error) {
	var bean T
	return GetEngine(ctx).ID(id).NoAutoCondition().NoAutoTime().Delete(&bean)
}

func DeleteByIDs[T any](ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}

	var bean T
	_, err := GetEngine(ctx).In("id", ids).NoAutoCondition().NoAutoTime().Delete(&bean)
	return err
}

func Delete[T any](ctx context.Context, opts FindOptions) (int64, error) {
	if opts == nil || !opts.ToConds().IsValid() {
		panic("opts are empty or invalid in db.Delete(ctx, opts). This should not be possible.")
	}

	var bean T
	return GetEngine(ctx).Where(opts.ToConds()).NoAutoCondition().NoAutoTime().Delete(&bean)
}

// DeleteByBean deletes all records according non-empty fields of the bean as conditions.
func DeleteByBean(ctx context.Context, bean any) (int64, error) {
	return GetEngine(ctx).Delete(bean)
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
	if len(ids) == 0 {
		return nil
	}
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
	return xormEngine.TableName(bean)
}

// InTransaction returns true if the engine is in a transaction otherwise return false
func InTransaction(ctx context.Context) bool {
	_, ok := inTransaction(ctx)
	return ok
}

func inTransaction(ctx context.Context) (*xorm.Session, bool) {
	e := getExistingEngine(ctx)
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
