// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// DBContext represents a db context
type DBContext struct {
	e Engine
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
