// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Context represents a db context
type Context struct {
	e Engine
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

// DefaultContext represents a Context with default Engine
func DefaultContext() *Context {
	return &Context{x}
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

	return &Context{sess}, sess, nil
}

// WithContext represents executing database operations
func WithContext(f func(ctx *Context) error) error {
	return f(&Context{x})
}

// WithTx represents executing database operations on a transaction
func WithTx(f func(ctx *Context) error) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := f(&Context{sess}); err != nil {
		return err
	}

	return sess.Commit()
}

// Iterate iterates the databases and doing something
func Iterate(ctx *Context, tableBean interface{}, cond builder.Cond, fun func(idx int, bean interface{}) error) error {
	return ctx.e.Where(cond).
		BufferSize(setting.Database.IterateBufferSize).
		Iterate(tableBean, fun)
}

// Insert inserts records into database
func Insert(ctx *Context, beans ...interface{}) error {
	_, err := ctx.e.Insert(beans...)
	return err
}
