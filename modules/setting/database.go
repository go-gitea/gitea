// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// SupportedDatabaseTypes includes all XORM supported databases type, sqlite3 maybe added by `database_sqlite3.go`
	SupportedDatabaseTypes = []string{"mysql", "postgres", "mssql"}
	// DatabaseTypeNames contains the friendly names for all database types
	DatabaseTypeNames = map[string]string{"mysql": "MySQL", "postgres": "PostgreSQL", "mssql": "MSSQL", "sqlite3": "SQLite3"}

	// EnableSQLite3 use SQLite3, set by build flag
	EnableSQLite3 bool

	// Database holds the database settings
	Database = struct {
		Type               DatabaseType
		Host               string
		Name               string
		User               string
		Passwd             string
		Schema             string
		SSLMode            string
		Path               string
		LogSQL             bool
		MysqlCharset       string
		CharsetCollation   string
		Timeout            int // seconds
		SQLiteJournalMode  string
		DBConnectRetries   int
		DBConnectBackoff   time.Duration
		MaxIdleConns       int
		MaxOpenConns       int
		ConnMaxLifetime    time.Duration
		IterateBufferSize  int
		AutoMigration      bool
		SlowQueryThreshold time.Duration
	}{
		Timeout:           500,
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
	if len(Database.Passwd) == 0 {
		Database.Passwd = sec.Key("PASSWD").String()
	}
	Database.Schema = sec.Key("SCHEMA").String()
	Database.SSLMode = sec.Key("SSL_MODE").MustString("disable")
	Database.CharsetCollation = sec.Key("CHARSET_COLLATION").String()

	Database.Path = sec.Key("PATH").MustString(filepath.Join(AppDataPath, "gitea.db"))
	Database.Timeout = sec.Key("SQLITE_TIMEOUT").MustInt(500)
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

// DBConnStr returns database connection string
func DBConnStr() (string, error) {
	var connStr string
	paramSep := "?"
	if strings.Contains(Database.Name, paramSep) {
		paramSep = "&"
	}
	switch Database.Type {
	case "mysql":
		connType := "tcp"
		if len(Database.Host) > 0 && Database.Host[0] == '/' { // looks like a unix socket
			connType = "unix"
		}
		tls := Database.SSLMode
		if tls == "disable" { // allow (Postgres-inspired) default value to work in MySQL
			tls = "false"
		}
		connStr = fmt.Sprintf("%s:%s@%s(%s)/%s%sparseTime=true&tls=%s",
			Database.User, Database.Passwd, connType, Database.Host, Database.Name, paramSep, tls)
	case "postgres":
		connStr = getPostgreSQLConnectionString(Database.Host, Database.User, Database.Passwd, Database.Name, Database.SSLMode)
	case "mssql":
		host, port := ParseMSSQLHostPort(Database.Host)
		connStr = fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;", host, port, Database.Name, Database.User, Database.Passwd)
	case "sqlite3":
		if !EnableSQLite3 {
			return "", errors.New("this Gitea binary was not built with SQLite3 support")
		}
		if err := os.MkdirAll(filepath.Dir(Database.Path), os.ModePerm); err != nil {
			return "", fmt.Errorf("Failed to create directories: %w", err)
		}
		journalMode := ""
		if Database.SQLiteJournalMode != "" {
			journalMode = "&_journal_mode=" + Database.SQLiteJournalMode
		}
		connStr = fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=%d&_txlock=immediate%s",
			Database.Path, Database.Timeout, journalMode)
	default:
		return "", fmt.Errorf("unknown database type: %s", Database.Type)
	}

	return connStr, nil
}

// parsePostgreSQLHostPort parses given input in various forms defined in
// https://www.postgresql.org/docs/current/static/libpq-connect.html#LIBPQ-CONNSTRING
// and returns proper host and port number.
func parsePostgreSQLHostPort(info string) (host, port string) {
	if h, p, err := net.SplitHostPort(info); err == nil {
		host, port = h, p
	} else {
		// treat the "info" as "host", if it's an IPv6 address, remove the wrapper
		host = info
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
	}

	// set fallback values
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	return host, port
}

func getPostgreSQLConnectionString(dbHost, dbUser, dbPasswd, dbName, dbsslMode string) (connStr string) {
	dbName, dbParam, _ := strings.Cut(dbName, "?")
	host, port := parsePostgreSQLHostPort(dbHost)
	connURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(dbUser, dbPasswd),
		Host:     net.JoinHostPort(host, port),
		Path:     dbName,
		OmitHost: false,
		RawQuery: dbParam,
	}
	query := connURL.Query()
	if strings.HasPrefix(host, "/") { // looks like a unix socket
		query.Add("host", host)
		connURL.Host = ":" + port
	}
	query.Set("sslmode", dbsslMode)
	connURL.RawQuery = query.Encode()
	return connURL.String()
}

// ParseMSSQLHostPort splits the host into host and port
func ParseMSSQLHostPort(info string) (string, string) {
	// the default port "0" might be related to MSSQL's dynamic port, maybe it should be double-confirmed in the future
	host, port := "127.0.0.1", "0"
	if strings.Contains(info, ":") {
		host = strings.Split(info, ":")[0]
		port = strings.Split(info, ":")[1]
	} else if strings.Contains(info, ",") {
		host = strings.Split(info, ",")[0]
		port = strings.TrimSpace(strings.Split(info, ",")[1])
	} else if len(info) > 0 {
		host = info
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "0"
	}
	return host, port
}

type DatabaseType string

func (t DatabaseType) String() string {
	return string(t)
}

func (t DatabaseType) IsSQLite3() bool {
	return t == "sqlite3"
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
