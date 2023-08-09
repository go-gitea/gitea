// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"

	_ "github.com/denisenkom/go-mssqldb" // Needed for the MSSQL driver
	_ "github.com/go-sql-driver/mysql"   // Needed for the MySQL driver
	_ "github.com/lib/pq"                // Needed for the Postgresql driver
)

var (
	x         *xorm.Engine
	tables    []any
	initFuncs []func() error

	// HasEngine specifies if we have a xorm.Engine
	HasEngine bool
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
	Sync2(...any) error
	Select(string) *xorm.Session
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
	return x.TableInfo(v)
}

// DumpTables dump tables information
func DumpTables(tables []*schemas.Table, w io.Writer, tp ...schemas.DBType) error {
	return x.DumpTables(tables, w, tp...)
}

// RegisterModel registers model, if initfunc provided, it will be invoked after data model sync
func RegisterModel(bean any, initFunc ...func() error) {
	tables = append(tables, bean)
	if len(initFuncs) > 0 && initFunc[0] != nil {
		initFuncs = append(initFuncs, initFunc[0])
	}
}

func init() {
	gonicNames := []string{"SSL", "UID"}
	for _, name := range gonicNames {
		names.LintGonicMapper[name] = true
	}
}

// newXORMEngine returns a new XORM engine from the configuration
func newXORMEngine() (*xorm.Engine, error) {
	connStr, err := setting.DBConnStr()
	if err != nil {
		return nil, err
	}

	var engine *xorm.Engine

	if setting.Database.Type.IsPostgreSQL() && len(setting.Database.Schema) > 0 {
		// OK whilst we sort out our schema issues - create a schema aware postgres
		registerPostgresSchemaDriver()
		engine, err = xorm.NewEngine("postgresschema", connStr)
	} else {
		engine, err = xorm.NewEngine(setting.Database.Type.String(), connStr)
	}

	if err != nil {
		return nil, err
	}
	if setting.Database.Type == "mysql" {
		engine.Dialect().SetParams(map[string]string{"rowFormat": "DYNAMIC"})
	} else if setting.Database.Type == "mssql" {
		engine.Dialect().SetParams(map[string]string{"DEFAULT_VARCHAR": "nvarchar"})
	}
	engine.SetSchema(setting.Database.Schema)
	return engine, nil
}

// SyncAllTables sync the schemas of all tables, is required by unit test code
func SyncAllTables() error {
	_, err := x.StoreEngine("InnoDB").SyncWithOptions(xorm.SyncOptions{
		WarnIfDatabaseColumnMissed: true,
	}, tables...)
	return err
}

// InitEngine initializes the xorm.Engine and sets it as db.DefaultContext
func InitEngine(ctx context.Context) error {
	xormEngine, err := newXORMEngine()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	xormEngine.SetMapper(names.GonicMapper{})
	// WARNING: for serv command, MUST remove the output to os.stdout,
	// so use log file to instead print to stdout.
	xormEngine.SetLogger(NewXORMLogger(setting.Database.LogSQL))
	xormEngine.ShowSQL(setting.Database.LogSQL)
	xormEngine.SetMaxOpenConns(setting.Database.MaxOpenConns)
	xormEngine.SetMaxIdleConns(setting.Database.MaxIdleConns)
	xormEngine.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)
	xormEngine.SetDefaultContext(ctx)

	SetDefaultEngine(ctx, xormEngine)
	return nil
}

// SetDefaultEngine sets the default engine for db
func SetDefaultEngine(ctx context.Context, eng *xorm.Engine) {
	x = eng
	DefaultContext = &Context{
		Context: ctx,
		e:       x,
	}
}

// UnsetDefaultEngine closes and unsets the default engine
// We hope the SetDefaultEngine and UnsetDefaultEngine can be paired, but it's impossible now,
// there are many calls to InitEngine -> SetDefaultEngine directly to overwrite the `x` and DefaultContext without close
// Global database engine related functions are all racy and there is no graceful close right now.
func UnsetDefaultEngine() {
	if x != nil {
		_ = x.Close()
		x = nil
	}
	DefaultContext = nil
}

// InitEngineWithMigration initializes a new xorm.Engine and sets it as the db.DefaultContext
// This function must never call .Sync2() if the provided migration function fails.
// When called from the "doctor" command, the migration function is a version check
// that prevents the doctor from fixing anything in the database if the migration level
// is different from the expected value.
func InitEngineWithMigration(ctx context.Context, migrateFunc func(*xorm.Engine) error) (err error) {
	if err = InitEngine(ctx); err != nil {
		return err
	}

	if err = x.Ping(); err != nil {
		return err
	}

	// We have to run migrateFunc here in case the user is re-running installation on a previously created DB.
	// If we do not then table schemas will be changed and there will be conflicts when the migrations run properly.
	//
	// Installation should only be being re-run if users want to recover an old database.
	// However, we should think carefully about should we support re-install on an installed instance,
	// as there may be other problems due to secret reinitialization.
	if err = migrateFunc(x); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err = SyncAllTables(); err != nil {
		return fmt.Errorf("sync database struct error: %w", err)
	}

	for _, initFunc := range initFuncs {
		if err := initFunc(); err != nil {
			return fmt.Errorf("initFunc failed: %w", err)
		}
	}

	return nil
}

// NamesToBean return a list of beans or an error
func NamesToBean(names ...string) ([]any, error) {
	beans := []any{}
	if len(names) == 0 {
		beans = append(beans, tables...)
		return beans, nil
	}
	// Need to map provided names to beans...
	beanMap := make(map[string]any)
	for _, bean := range tables {

		beanMap[strings.ToLower(reflect.Indirect(reflect.ValueOf(bean)).Type().Name())] = bean
		beanMap[strings.ToLower(x.TableName(bean))] = bean
		beanMap[strings.ToLower(x.TableName(bean, true))] = bean
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

// DumpDatabase dumps all data from database according the special database SQL syntax to file system.
func DumpDatabase(filePath, dbType string) error {
	var tbs []*schemas.Table
	for _, t := range tables {
		t, err := x.TableInfo(t)
		if err != nil {
			return err
		}
		tbs = append(tbs, t)
	}

	type Version struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}
	t, err := x.TableInfo(&Version{})
	if err != nil {
		return err
	}
	tbs = append(tbs, t)

	if len(dbType) > 0 {
		return x.DumpTablesToFile(tbs, filePath, schemas.DBType(dbType))
	}
	return x.DumpTablesToFile(tbs, filePath)
}

// MaxBatchInsertSize returns the table's max batch insert size
func MaxBatchInsertSize(bean any) int {
	t, err := x.TableInfo(bean)
	if err != nil {
		return 50
	}
	return 999 / len(t.ColumnsSeq())
}

// IsTableNotEmpty returns true if table has at least one record
func IsTableNotEmpty(tableName string) (bool, error) {
	return x.Table(tableName).Exist()
}

// DeleteAllRecords will delete all the records of this table
func DeleteAllRecords(tableName string) error {
	_, err := x.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	return err
}

// GetMaxID will return max id of the table
func GetMaxID(beanOrTableName any) (maxID int64, err error) {
	_, err = x.Select("MAX(id)").Table(beanOrTableName).Get(&maxID)
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
