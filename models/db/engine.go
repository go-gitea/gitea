// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	// Needed for the MySQL driver
	_ "github.com/go-sql-driver/mysql"

	// Needed for the Postgresql driver
	_ "github.com/lib/pq"

	// Needed for the MSSQL driver
	_ "github.com/denisenkom/go-mssqldb"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var (
	x         *xorm.Engine
	tables    []interface{}
	initFuncs []func() error

	// HasEngine specifies if we have a xorm.Engine
	HasEngine bool
)

// Engine represents a xorm engine or session.
type Engine interface {
	Table(tableNameOrBean interface{}) *xorm.Session
	Count(...interface{}) (int64, error)
	Decr(column string, arg ...interface{}) *xorm.Session
	Delete(...interface{}) (int64, error)
	Exec(...interface{}) (sql.Result, error)
	Find(interface{}, ...interface{}) error
	Get(interface{}) (bool, error)
	ID(interface{}) *xorm.Session
	In(string, ...interface{}) *xorm.Session
	Incr(column string, arg ...interface{}) *xorm.Session
	Insert(...interface{}) (int64, error)
	InsertOne(interface{}) (int64, error)
	Iterate(interface{}, xorm.IterFunc) error
	Join(joinOperator string, tablename interface{}, condition string, args ...interface{}) *xorm.Session
	SQL(interface{}, ...interface{}) *xorm.Session
	Where(interface{}, ...interface{}) *xorm.Session
	Asc(colNames ...string) *xorm.Session
	Desc(colNames ...string) *xorm.Session
	Limit(limit int, start ...int) *xorm.Session
	SumInt(bean interface{}, columnName string) (res int64, err error)
	Sync2(...interface{}) error
	Select(string) *xorm.Session
	NotIn(string, ...interface{}) *xorm.Session
	OrderBy(string) *xorm.Session
	Exist(...interface{}) (bool, error)
	Distinct(...string) *xorm.Session
	Query(...interface{}) ([]map[string][]byte, error)
	Cols(...string) *xorm.Session
}

// RegisterModel registers model, if initFunc provided, it will be invoked after data model sync
func RegisterModel(bean interface{}, initFunc ...func() error) {
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

// NewEngine returns a new xorm engine from the configuration
func NewEngine() (*xorm.Engine, error) {
	connStr, err := setting.DBConnStr()
	if err != nil {
		return nil, err
	}

	var engine *xorm.Engine

	if setting.Database.UsePostgreSQL && len(setting.Database.Schema) > 0 {
		// OK whilst we sort out our schema issues - create a schema aware postgres
		registerPostgresSchemaDriver()
		engine, err = xorm.NewEngine("postgresschema", connStr)
	} else {
		engine, err = xorm.NewEngine(setting.Database.Type, connStr)
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

func syncTables() error {
	return x.StoreEngine("InnoDB").Sync2(tables...)
}

// InitInstallEngineWithMigration creates a new xorm.Engine for testing during install
//
// This function will cause the basic database schema to be created
func InitInstallEngineWithMigration(ctx context.Context, migrateFunc func(*xorm.Engine) error) (err error) {
	x, err = NewEngine()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	x.SetMapper(names.GonicMapper{})
	x.SetLogger(NewXORMLogger(!setting.IsProd))
	x.ShowSQL(!setting.IsProd)

	x.SetDefaultContext(ctx)

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
		return fmt.Errorf("migrate: %v", err)
	}

	return syncTables()
}

// InitEngine sets the xorm.Engine
func InitEngine() (err error) {
	x, err = NewEngine()
	if err != nil {
		return fmt.Errorf("Failed to connect to database: %v", err)
	}

	x.SetMapper(names.GonicMapper{})
	// WARNING: for serv command, MUST remove the output to os.stdout,
	// so use log file to instead print to stdout.
	x.SetLogger(NewXORMLogger(setting.Database.LogSQL))
	x.ShowSQL(setting.Database.LogSQL)
	x.SetMaxOpenConns(setting.Database.MaxOpenConns)
	x.SetMaxIdleConns(setting.Database.MaxIdleConns)
	x.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)
	return nil
}

// InitEngineWithMigration initializes a new xorm.Engine
// This function must never call .Sync2() if the provided migration function fails.
// When called from the "doctor" command, the migration function is a version check
// that prevents the doctor from fixing anything in the database if the migration level
// is different from the expected value.
func InitEngineWithMigration(ctx context.Context, migrateFunc func(*xorm.Engine) error) (err error) {
	if err = InitEngine(); err != nil {
		return err
	}

	DefaultContext = &Context{
		Context: ctx,
		e:       x,
	}

	x.SetDefaultContext(ctx)

	if err = x.Ping(); err != nil {
		return err
	}

	if err = migrateFunc(x); err != nil {
		return fmt.Errorf("migrate: %v", err)
	}

	if err = syncTables(); err != nil {
		return fmt.Errorf("sync database struct error: %v", err)
	}

	for _, initFunc := range initFuncs {
		if err := initFunc(); err != nil {
			return fmt.Errorf("initFunc failed: %v", err)
		}
	}

	return nil
}

// TablesToBeans return a list of beans for tables or an error
func TablesToBeans(names ...string) ([]interface{}, error) {
	var beans []interface{}
	if len(names) == 0 {
		beans = append(beans, tables...)
		return beans, nil
	}
	// Need to map provided names to beans ...
	beanMap := make(map[string]interface{})
	for _, bean := range tables {
		beanMap[strings.ToLower(reflect.Indirect(reflect.ValueOf(bean)).Type().Name())] = bean
		beanMap[strings.ToLower(x.TableName(bean))] = bean
		beanMap[strings.ToLower(x.TableName(bean, true))] = bean
	}

	gotBean := make(map[interface{}]bool)
	for _, name := range names {
		bean, ok := beanMap[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("No table found that matches: %s", name)
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
