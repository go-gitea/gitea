// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"

	_ "github.com/go-sql-driver/mysql"  // Needed for the MySQL driver
	_ "github.com/lib/pq"               // Needed for the Postgresql driver
	_ "github.com/microsoft/go-mssqldb" // Needed for the MSSQL driver
)

var (
	xormEngine          *xorm.Engine
	registeredModels    []any
	registeredInitFuncs []func() error
)

// Engine represents a xorm engine or session.
type Engine interface {
	Table(tableNameOrBean any) *xorm.Session
	Count(...any) (int64, error)
	Decr(column string, arg ...any) *xorm.Session
	Delete(...any) (int64, error)
	Truncate(...any) (int64, error)
	Exec(...any) (sql.Result, error)
	Find(any, ...any) error
	Get(beans ...any) (bool, error)
	ID(any) *xorm.Session
	In(string, ...any) *xorm.Session
	Incr(column string, arg ...any) *xorm.Session
	Insert(...any) (int64, error)
	Iterate(any, xorm.IterFunc) error
	Join(joinOperator string, tablename, condition any, args ...any) *xorm.Session
	SQL(any, ...any) *xorm.Session
	Where(any, ...any) *xorm.Session
	Asc(colNames ...string) *xorm.Session
	Desc(colNames ...string) *xorm.Session
	Limit(limit int, start ...int) *xorm.Session
	NoAutoTime() *xorm.Session
	SumInt(bean any, columnName string) (res int64, err error)
	Sync(...any) error
	Select(string) *xorm.Session
	SetExpr(string, any) *xorm.Session
	NotIn(string, ...any) *xorm.Session
	OrderBy(any, ...any) *xorm.Session
	Exist(...any) (bool, error)
	Distinct(...string) *xorm.Session
	Query(...any) ([]map[string][]byte, error)
	Cols(...string) *xorm.Session
	Context(ctx context.Context) *xorm.Session
	Ping() error
}

// TableInfo returns table's information via an object
func TableInfo(v any) (*schemas.Table, error) {
	return xormEngine.TableInfo(v)
}

// RegisterModel registers model, if initFuncs provided, it will be invoked after data model sync
func RegisterModel(bean any, initFunc ...func() error) {
	registeredModels = append(registeredModels, bean)
	if len(registeredInitFuncs) > 0 && initFunc[0] != nil {
		registeredInitFuncs = append(registeredInitFuncs, initFunc[0])
	}
}

// SyncAllTables sync the schemas of all tables, is required by unit test code
func SyncAllTables() error {
	_, err := xormEngine.StoreEngine("InnoDB").SyncWithOptions(xorm.SyncOptions{
		WarnIfDatabaseColumnMissed: true,
	}, registeredModels...)
	return err
}

// NamesToBean return a list of beans or an error
func NamesToBean(names ...string) ([]any, error) {
	beans := []any{}
	if len(names) == 0 {
		beans = append(beans, registeredModels...)
		return beans, nil
	}
	// Need to map provided names to beans...
	beanMap := make(map[string]any)
	for _, bean := range registeredModels {
		beanMap[strings.ToLower(reflect.Indirect(reflect.ValueOf(bean)).Type().Name())] = bean
		beanMap[strings.ToLower(xormEngine.TableName(bean))] = bean
		beanMap[strings.ToLower(xormEngine.TableName(bean, true))] = bean
	}

	gotBean := make(map[any]bool)
	for _, name := range names {
		bean, ok := beanMap[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("no table found that matches: %s", name)
		}
		if !gotBean[bean] {
			beans = append(beans, bean)
			gotBean[bean] = true
		}
	}
	return beans, nil
}

// MaxBatchInsertSize returns the table's max batch insert size
func MaxBatchInsertSize(bean any) int {
	t, err := xormEngine.TableInfo(bean)
	if err != nil {
		return 50
	}
	return 999 / len(t.ColumnsSeq())
}

// IsTableNotEmpty returns true if table has at least one record
func IsTableNotEmpty(beanOrTableName any) (bool, error) {
	return xormEngine.Table(beanOrTableName).Exist()
}

// DeleteAllRecords will delete all the records of this table
func DeleteAllRecords(tableName string) error {
	_, err := xormEngine.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	return err
}

// GetMaxID will return max id of the table
func GetMaxID(beanOrTableName any) (maxID int64, err error) {
	_, err = xormEngine.Select("MAX(id)").Table(beanOrTableName).Get(&maxID)
	return maxID, err
}

func SetLogSQL(ctx context.Context, on bool) {
	e := GetEngine(ctx)
	if x, ok := e.(*xorm.Engine); ok {
		x.ShowSQL(on)
	} else if sess, ok := e.(*xorm.Session); ok {
		sess.Engine().ShowSQL(on)
	}
}
