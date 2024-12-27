// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"xorm.io/xorm"
	"xorm.io/xorm/core"
	"xorm.io/xorm/names"
)

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
	engineName := setting.Database.Type.String()

	if setting.Database.Type.IsPostgreSQL() && len(setting.Database.Schema) > 0 {
		// OK whilst we sort out our schema issues - create a schema aware postgres
		registerPostgresSchemaDriver()
		engineName = "postgresschema"
	}

	var semconvDBSystem attribute.KeyValue
	switch setting.Database.Type {
	case "sqlite3":
		semconvDBSystem = semconv.DBSystemSqlite
	case "mysql":
		semconvDBSystem = semconv.DBSystemMySQL
	case "postgres":
		semconvDBSystem = semconv.DBSystemPostgreSQL
	case "mssql":
		semconvDBSystem = semconv.DBSystemMSSQL
	default:
		return nil, fmt.Errorf("unsupported database type: %s", setting.Database.Type)
	}

	db, err := otelsql.Open(engineName, connStr,
		otelsql.WithAttributes(semconvDBSystem),
		otelsql.WithDBName(setting.Database.Name))
	if err != nil {
		return engine, err
	}
	xormCore := core.FromDB(db)
	engine, err = xorm.NewEngineWithDB(engineName, connStr, xormCore)
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

// InitEngine initializes the xorm.Engine and sets it as db.DefaultContext
func InitEngine(ctx context.Context) error {
	xe, err := newXORMEngine()
	if err != nil {
		if strings.Contains(err.Error(), "SQLite3 support") {
			return fmt.Errorf(`sqlite3 requires: -tags sqlite,sqlite_unlock_notify%s%w`, "\n", err)
		}
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	xe.SetMapper(names.GonicMapper{})
	// WARNING: for serv command, MUST remove the output to os.stdout,
	// so use log file to instead print to stdout.
	xe.SetLogger(NewXORMLogger(setting.Database.LogSQL))
	xe.ShowSQL(setting.Database.LogSQL)
	xe.SetMaxOpenConns(setting.Database.MaxOpenConns)
	xe.SetMaxIdleConns(setting.Database.MaxIdleConns)
	xe.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)
	xe.SetDefaultContext(ctx)

	if setting.Database.SlowQueryThreshold > 0 {
		xe.AddHook(&SlowQueryHook{
			Threshold: setting.Database.SlowQueryThreshold,
			Logger:    log.GetLogger("xorm"),
		})
	}

	SetDefaultEngine(ctx, xe)
	return nil
}

// SetDefaultEngine sets the default engine for db
func SetDefaultEngine(ctx context.Context, eng *xorm.Engine) {
	xormEngine = eng
	DefaultContext = &Context{Context: ctx, engine: xormEngine}
}

// UnsetDefaultEngine closes and unsets the default engine
// We hope the SetDefaultEngine and UnsetDefaultEngine can be paired, but it's impossible now,
// there are many calls to InitEngine -> SetDefaultEngine directly to overwrite the `xormEngine` and DefaultContext without close
// Global database engine related functions are all racy and there is no graceful close right now.
func UnsetDefaultEngine() {
	if xormEngine != nil {
		_ = xormEngine.Close()
		xormEngine = nil
	}
	DefaultContext = nil
}

// InitEngineWithMigration initializes a new xorm.Engine and sets it as the db.DefaultContext
// This function must never call .Sync() if the provided migration function fails.
// When called from the "doctor" command, the migration function is a version check
// that prevents the doctor from fixing anything in the database if the migration level
// is different from the expected value.
func InitEngineWithMigration(ctx context.Context, migrateFunc func(*xorm.Engine) error) (err error) {
	if err = InitEngine(ctx); err != nil {
		return err
	}

	if err = xormEngine.Ping(); err != nil {
		return err
	}

	preprocessDatabaseCollation(xormEngine)

	// We have to run migrateFunc here in case the user is re-running installation on a previously created DB.
	// If we do not then table schemas will be changed and there will be conflicts when the migrations run properly.
	//
	// Installation should only be being re-run if users want to recover an old database.
	// However, we should think carefully about should we support re-install on an installed instance,
	// as there may be other problems due to secret reinitialization.
	if err = migrateFunc(xormEngine); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err = SyncAllTables(); err != nil {
		return fmt.Errorf("sync database struct error: %w", err)
	}

	for _, initFunc := range registeredInitFuncs {
		if err := initFunc(); err != nil {
			return fmt.Errorf("initFunc failed: %w", err)
		}
	}

	return nil
}
