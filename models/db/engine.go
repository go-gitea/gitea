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

	_ "github.com/go-sql-driver/mysql"  // Needed for the MySQL driver
	_ "github.com/lib/pq"               // Needed for the Postgresql driver
	_ "github.com/microsoft/go-mssqldb" // Needed for the MSSQL driver

	"xorm.io/xorm"
	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var (
	xormEngine          *xorm.Engine
	registeredModels    []any
	registeredInitFuncs []func() error
)

// SQLSession represents a common interface for engine and session to execute SQLs
type SQLSession interface {
	Count(...any) (int64, error)
	Decr(column string, arg ...any) *xorm.Session
	Delete(...any) (int64, error)
	Truncate(...any) (int64, error)
	Exec(...any) (sql.Result, error)
	Find(any, ...any) error
	FindAndCount(any, ...any) (int64, error)
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
	Select(string) *xorm.Session
	SetExpr(string, any) *xorm.Session
	NotIn(string, ...any) *xorm.Session
	OrderBy(any, ...any) *xorm.Session
	Exist(...any) (bool, error)
	Distinct(...string) *xorm.Session
	Query(...any) ([]map[string][]byte, error)
	Cols(...string) *xorm.Session
	Table(tableNameOrBean any) *xorm.Session
	Context(ctx context.Context) *xorm.Session
	QueryInterface(sqlOrArgs ...any) ([]map[string]any, error)
	IsTableExist(tableNameOrBean any) (bool, error)
}

// Engine represents a xorm engine
type Engine interface {
	SQLSession
	Sync(...any) error
	Ping() error
}

// Session represents a xorm session interface
type Session interface {
	Engine
	And(query any, args ...any) *xorm.Session
	Begin() error
	Close() error
	Commit() error
	IsInTx() bool
	Rollback() error
	Engine() *xorm.Engine
}

// EngineMigration is a xorm engine interface used for migrations.
// It extends Engine with additional methods that are only available on the engine (not on the session)
// and are needed by the migration packages.
type EngineMigration interface {
	Engine
	Close() error
	DB() *core.DB
	DBMetas() ([]*schemas.Table, error)
	Dialect() dialects.Dialect
	DropTables(beans ...any) error
	NewSession() *xorm.Session
	SetMapper(mapper names.Mapper)
	SyncWithOptions(opts xorm.SyncOptions, beans ...any) (*xorm.SyncResult, error)
	TableInfo(bean any) (*schemas.Table, error)
	TableName(bean any, includeSchema ...bool) string
}

var (
	_ Engine          = (*xorm.Engine)(nil)
	_ Engine          = (*xorm.Session)(nil)
	_ Session         = (*xorm.Session)(nil)
	_ EngineMigration = (*xorm.Engine)(nil)
)

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
	_, err := xormEngine.Exec("DELETE FROM " + tableName)
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
