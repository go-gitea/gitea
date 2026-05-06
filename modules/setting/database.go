// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"time"
)

const defaultSQLiteBusyTimeout = 20 * 1000

var (
	// SupportedDatabaseTypes includes all XORM supported databases type, sqlite3 maybe added by the tag-controlled drivers
	SupportedDatabaseTypes = []string{"mysql", "postgres", "mssql"}
	// DatabaseTypeNames contains the friendly names for all database types
	DatabaseTypeNames = map[string]string{"mysql": "MySQL", "postgres": "PostgreSQL", "mssql": "MSSQL", DatabaseTypeSQLite3: "SQLite3"}

	// Database holds the database settings
	Database = struct {
		Type    DatabaseType
		Host    string
		Name    string
		User    string
		Passwd  string
		Schema  string
		SSLMode string
		Path    string

		SQLiteBusyTimeout int
		SQLiteJournalMode string

		LogSQL             bool
		CharsetCollation   string
		DBConnectRetries   int
		DBConnectBackoff   time.Duration
		MaxIdleConns       int
		MaxOpenConns       int
		ConnMaxLifetime    time.Duration
		IterateBufferSize  int
		AutoMigration      bool
		SlowQueryThreshold time.Duration
	}{
		IterateBufferSize: 50,
	}
)

// LoadDBSetting loads the database settings
func LoadDBSetting() {
	loadDBSetting(CfgProvider)
}

func loadDBSetting(rootCfg ConfigProvider) {
	sec := rootCfg.Section("database")
	Database.Type = DatabaseType(sec.Key("DB_TYPE").String())

	Database.Host = sec.Key("HOST").String()
	Database.Name = sec.Key("NAME").String()
	Database.User = sec.Key("USER").String()
	Database.Passwd = sec.Key("PASSWD").String()

	Database.Schema = sec.Key("SCHEMA").String()
	Database.SSLMode = sec.Key("SSL_MODE").MustString("disable")
	Database.CharsetCollation = sec.Key("CHARSET_COLLATION").String()

	Database.Path = sec.Key("PATH").MustString(filepath.Join(AppDataPath, "gitea.db"))

	Database.SQLiteBusyTimeout = sec.Key("SQLITE_TIMEOUT").MustInt(defaultSQLiteBusyTimeout)
	// mattn driver isn't really affected by this timeout, but other drivers are affected
	// the default value was 500 (0.5s), to avoid breaking existing users, make sure the timeout is long enough (at least, 5 seconds)
	if Database.SQLiteBusyTimeout < 5000 {
		Database.SQLiteBusyTimeout = defaultSQLiteBusyTimeout
	}
	Database.SQLiteJournalMode = sec.Key("SQLITE_JOURNAL_MODE").MustString("")

	Database.MaxIdleConns = sec.Key("MAX_IDLE_CONNS").MustInt(2)
	if Database.Type.IsMySQL() {
		Database.ConnMaxLifetime = sec.Key("CONN_MAX_LIFETIME").MustDuration(3 * time.Second)
	} else {
		Database.ConnMaxLifetime = sec.Key("CONN_MAX_LIFETIME").MustDuration(0)
	}
	Database.MaxOpenConns = sec.Key("MAX_OPEN_CONNS").MustInt(0)

	Database.IterateBufferSize = sec.Key("ITERATE_BUFFER_SIZE").MustInt(50)
	Database.LogSQL = sec.Key("LOG_SQL").MustBool(false)
	Database.DBConnectRetries = sec.Key("DB_RETRIES").MustInt(10)
	Database.DBConnectBackoff = sec.Key("DB_RETRY_BACKOFF").MustDuration(3 * time.Second)
	Database.AutoMigration = sec.Key("AUTO_MIGRATION").MustBool(true)
	Database.SlowQueryThreshold = sec.Key("SLOW_QUERY_THRESHOLD").MustDuration(5 * time.Second)
}

// DatabaseType FIXME: it is also used directly with "schemas.DBType", so the names must be consistent
type DatabaseType string

const DatabaseTypeSQLite3 = "sqlite3"

func (t DatabaseType) IsSQLite3() bool {
	return t == DatabaseTypeSQLite3
}

func (t DatabaseType) IsMySQL() bool {
	return t == "mysql"
}

func (t DatabaseType) IsMSSQL() bool {
	return t == "mssql"
}

func (t DatabaseType) IsPostgreSQL() bool {
	return t == "postgres"
}
