// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "xorm.io/builder"

// DBContext represents a db context
type DBContext struct {
	e Engine
}

// Insert inserts a object to database
func (ctx *DBContext) Insert(obj interface{}) error {
	_, err := ctx.e.Insert(obj)
	return err
}

// LoadByID loads record from database according id, if it's not exist return ErrNotExist
func (ctx *DBContext) LoadByID(id int64, obj interface{}) error {
	has, err := ctx.e.ID(id).Get(obj)
	if err != nil {
		return err
	} else if !has {
		return ErrNotExist{ID: id}
	}
	return nil
}

// FindByConditions loads records by conditions
func (ctx *DBContext) FindByConditions(conds builder.Cond, objs interface{}) error {
	return ctx.e.Where(conds).Find(objs)
}

// DeleteByID deletes a object by id
func (ctx *DBContext) DeleteByID(id int64, obj interface{}) error {
	_, err := ctx.e.ID(id).NoAutoCondition().Delete(obj)
	return err
}

// UpdateByID updates a record
func (ctx *DBContext) UpdateByID(id int64, obj interface{}, cols ...string) error {
	sess := ctx.e.ID(id)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(obj)
	return err
}

// WithContext represents executing database operations
func WithContext(f func(ctx DBContext) error) error {
	return f(DBContext{x})
}

// WithTransaction represents executing database operations on a trasaction
func WithTransaction(f func(ctx DBContext) error) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		sess.Close()
		return err
	}

	if err := f(DBContext{sess}); err != nil {
		sess.Close()
		return err
	}

	err := sess.Commit()
	sess.Close()
	return err
}
