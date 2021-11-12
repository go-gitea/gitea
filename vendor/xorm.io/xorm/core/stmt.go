// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import (
	"context"
	"database/sql"
	"errors"
	"reflect"

	"xorm.io/xorm/contexts"
)

// Stmt reprents a stmt objects
type Stmt struct {
	*sql.Stmt
	db    *DB
	names map[string]int
	query string
}

// PrepareContext creates a prepare statement
func (db *DB) PrepareContext(ctx context.Context, query string) (*Stmt, error) {
	names := make(map[string]int)
	var i int
	query = re.ReplaceAllStringFunc(query, func(src string) string {
		names[src[1:]] = i
		i++
		return "?"
	})
	hookCtx := contexts.NewContextHook(ctx, "PREPARE", nil)
	ctx, err := db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	stmt, err := db.DB.PrepareContext(ctx, query)
	hookCtx.End(ctx, nil, err)
	if err := db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return &Stmt{stmt, db, names, query}, nil
}

// Prepare creates a prepare statement
func (db *DB) Prepare(query string) (*Stmt, error) {
	return db.PrepareContext(context.Background(), query)
}

// ExecMapContext execute with map
func (s *Stmt) ExecMapContext(ctx context.Context, mp interface{}) (sql.Result, error) {
	vv := reflect.ValueOf(mp)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Map {
		return nil, errors.New("mp should be a map's pointer")
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().MapIndex(reflect.ValueOf(k)).Interface()
	}
	return s.ExecContext(ctx, args...)
}

// ExecMap executes with map
func (s *Stmt) ExecMap(mp interface{}) (sql.Result, error) {
	return s.ExecMapContext(context.Background(), mp)
}

// ExecStructContext executes with struct
func (s *Stmt) ExecStructContext(ctx context.Context, st interface{}) (sql.Result, error) {
	vv := reflect.ValueOf(st)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Struct {
		return nil, errors.New("mp should be a map's pointer")
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().FieldByName(k).Interface()
	}
	return s.ExecContext(ctx, args...)
}

// ExecStruct executes with struct
func (s *Stmt) ExecStruct(st interface{}) (sql.Result, error) {
	return s.ExecStructContext(context.Background(), st)
}

// ExecContext with args
func (s *Stmt) ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error) {
	hookCtx := contexts.NewContextHook(ctx, s.query, args)
	ctx, err := s.db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	res, err := s.Stmt.ExecContext(ctx, args)
	hookCtx.End(ctx, res, err)
	if err := s.db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return res, nil
}

// QueryContext query with args
func (s *Stmt) QueryContext(ctx context.Context, args ...interface{}) (*Rows, error) {
	hookCtx := contexts.NewContextHook(ctx, s.query, args)
	ctx, err := s.db.beforeProcess(hookCtx)
	if err != nil {
		return nil, err
	}
	rows, err := s.Stmt.QueryContext(ctx, args...)
	hookCtx.End(ctx, nil, err)
	if err := s.db.afterProcess(hookCtx); err != nil {
		return nil, err
	}
	return &Rows{rows, s.db}, nil
}

// Query query with args
func (s *Stmt) Query(args ...interface{}) (*Rows, error) {
	return s.QueryContext(context.Background(), args...)
}

// QueryMapContext query with map
func (s *Stmt) QueryMapContext(ctx context.Context, mp interface{}) (*Rows, error) {
	vv := reflect.ValueOf(mp)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Map {
		return nil, errors.New("mp should be a map's pointer")
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().MapIndex(reflect.ValueOf(k)).Interface()
	}

	return s.QueryContext(ctx, args...)
}

// QueryMap query with map
func (s *Stmt) QueryMap(mp interface{}) (*Rows, error) {
	return s.QueryMapContext(context.Background(), mp)
}

// QueryStructContext query with struct
func (s *Stmt) QueryStructContext(ctx context.Context, st interface{}) (*Rows, error) {
	vv := reflect.ValueOf(st)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Struct {
		return nil, errors.New("mp should be a map's pointer")
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().FieldByName(k).Interface()
	}

	return s.QueryContext(ctx, args...)
}

// QueryStruct query with struct
func (s *Stmt) QueryStruct(st interface{}) (*Rows, error) {
	return s.QueryStructContext(context.Background(), st)
}

// QueryRowContext query row with args
func (s *Stmt) QueryRowContext(ctx context.Context, args ...interface{}) *Row {
	rows, err := s.QueryContext(ctx, args...)
	return &Row{rows, err}
}

// QueryRow query row with args
func (s *Stmt) QueryRow(args ...interface{}) *Row {
	return s.QueryRowContext(context.Background(), args...)
}

// QueryRowMapContext query row with map
func (s *Stmt) QueryRowMapContext(ctx context.Context, mp interface{}) *Row {
	vv := reflect.ValueOf(mp)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Map {
		return &Row{nil, errors.New("mp should be a map's pointer")}
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().MapIndex(reflect.ValueOf(k)).Interface()
	}

	return s.QueryRowContext(ctx, args...)
}

// QueryRowMap query row with map
func (s *Stmt) QueryRowMap(mp interface{}) *Row {
	return s.QueryRowMapContext(context.Background(), mp)
}

// QueryRowStructContext query row with struct
func (s *Stmt) QueryRowStructContext(ctx context.Context, st interface{}) *Row {
	vv := reflect.ValueOf(st)
	if vv.Kind() != reflect.Ptr || vv.Elem().Kind() != reflect.Struct {
		return &Row{nil, errors.New("st should be a struct's pointer")}
	}

	args := make([]interface{}, len(s.names))
	for k, i := range s.names {
		args[i] = vv.Elem().FieldByName(k).Interface()
	}

	return s.QueryRowContext(ctx, args...)
}

// QueryRowStruct query row with struct
func (s *Stmt) QueryRowStruct(st interface{}) *Row {
	return s.QueryRowStructContext(context.Background(), st)
}
