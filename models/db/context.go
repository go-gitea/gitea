// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"

	"code.gitea.io/gitea/modules/setting"

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

// EnginedContextKey is a context key. It is used with context.Value() to get the current Engined for the context
var EnginedContextKey = &contextKey{"engined"}

// Context represents a db context
type Context struct {
	context.Context
	e Engine
}

// WithEngine returns a db.Context from a context.Context and db.Engine
func WithEngine(ctx context.Context, e Engine) *Context {
	return &Context{
		Context: ctx,
		e:       e,
	}
}

// Engine returns db engine
func (ctx *Context) Engine() Engine {
	return ctx.e
}

// NewSession returns a new session
func (ctx *Context) NewSession() *xorm.Session {
	e, ok := ctx.e.(*xorm.Engine)
	if ok {
		return e.NewSession()
	}
	return nil
}

// Value shadows Value for context.Context but allows us to get ourselves and an Engined object
func (ctx *Context) Value(key interface{}) interface{} {
	if key == EnginedContextKey {
		return ctx
	}
	return ctx.Context.Value(key)
}

// Engined structs provide an Engine
type Engined interface {
	Engine() Engine
	NewSession() *xorm.Session
}

// GetEngine will get a db Engine from this context or return an Engine restricted to this context
func GetEngine(ctx context.Context) Engine {
	if engined, ok := ctx.(Engined); ok {
		return engined.Engine()
	}
	enginedInterface := ctx.Value(EnginedContextKey)
	if enginedInterface != nil {
		return enginedInterface.(Engined).Engine()
	}
	return x.Context(ctx)
}

// NewSession will get a db Session from this context or return a session restricted to this context
func NewSession(ctx context.Context) *xorm.Session {
	if engined, ok := ctx.(Engined); ok {
		return engined.NewSession()
	}

	enginedInterface := ctx.Value(EnginedContextKey)
	if enginedInterface != nil {
		sess := enginedInterface.(Engined).NewSession()
		if sess != nil {
			return sess.Context(ctx)
		}
		return nil
	}

	return x.NewSession().Context(ctx)
}

// Committer represents an interface to Commit or Close the Context
type Committer interface {
	Commit() error
	Close() error
}

// TxContext represents a transaction Context
func TxContext() (*Context, Committer, error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		sess.Close()
		return nil, nil, err
	}

	return &Context{
		Context: DefaultContext,
		e:       sess,
	}, sess, nil
}

// WithContext represents executing database operations
func WithContext(f func(ctx *Context) error) error {
	return f(&Context{
		Context: DefaultContext,
		e:       x,
	})
}

// WithTx represents executing database operations on a transaction
func WithTx(f func(ctx context.Context) error) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := f(&Context{
		Context: DefaultContext,
		e:       sess,
	}); err != nil {
		return err
	}

	return sess.Commit()
}

// Iterate iterates the databases and doing something
func Iterate(ctx context.Context, tableBean interface{}, cond builder.Cond, fun func(idx int, bean interface{}) error) error {
	return GetEngine(ctx).Where(cond).
		BufferSize(setting.Database.IterateBufferSize).
		Iterate(tableBean, fun)
}

// Insert inserts records into database
func Insert(ctx context.Context, beans ...interface{}) error {
	_, err := GetEngine(ctx).Insert(beans...)
	return err
}
