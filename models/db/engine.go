// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
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

func TableInfo(v interface{}) (*schemas.Table, error) {
	return x.TableInfo(v)
}

func DumpTables(tables []*schemas.Table, w io.Writer, tp ...schemas.DBType) error {
	return x.DumpTables(tables, w, tp...)
}

// InitEngine In case of problems connecting to DB, retry connection. Eg, PGSQL in Docker Container on Synology
func InitEngine(ctx context.Context) (err error) {
	log.Info("Beginning ORM engine initialization.")
	for i := 0; i < setting.Database.DBConnectRetries; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Aborted due to shutdown:\nin retry ORM engine initialization")
		default:
		}
		log.Info("ORM engine initialization attempt #%d/%d...", i+1, setting.Database.DBConnectRetries)
		if err = NewEngine(ctx, migrations.Migrate); err == nil {
			break
		} else if i == setting.Database.DBConnectRetries-1 {
			return err
		}
		log.Error("ORM engine initialization attempt #%d/%d failed. Error: %v", i+1, setting.Database.DBConnectRetries, err)
		log.Info("Backing off for %d seconds", int64(setting.Database.DBConnectBackoff/time.Second))
		time.Sleep(setting.Database.DBConnectBackoff)
	}
	HasEngine = true
	return nil
}
