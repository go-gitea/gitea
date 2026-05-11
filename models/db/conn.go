// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type ConnOptions struct {
	Type     setting.DatabaseType
	Host     string
	Database string
	User     string
	Passwd   string
	Schema   string
	SSLMode  string

	SQLitePath        string
	SQLiteBusyTimeout int
	SQLiteJournalMode string
}

type SQLiteConnStrOptions struct {
	FilePath string
	// how long a concurrent query can wait for others (milliseconds),
	// if timeout is reached, the error is something like "database is locked (SQLITE_BUSY)"
	BusyTimeout int
	JournalMode string
}

func GlobalConnOptions() ConnOptions {
	return ConnOptions{
		Type:     setting.Database.Type,
		Host:     setting.Database.Host,
		Database: setting.Database.Name,
		User:     setting.Database.User,
		Passwd:   setting.Database.Passwd,
		Schema:   setting.Database.Schema,
		SSLMode:  setting.Database.SSLMode,

		SQLitePath:        setting.Database.Path,
		SQLiteBusyTimeout: setting.Database.SQLiteBusyTimeout,
		SQLiteJournalMode: setting.Database.SQLiteJournalMode,
	}
}

const (
	sqlDriverPostgresSchema = "postgresschema"
	sqlDriverSQLite3        = "sqlite3" // although database type also has "sqlite3", they are different, for different purposes
)

var makeSQLiteConnStr = func(opts SQLiteConnStrOptions) (string, string, error) {
	return "", "", errors.New(`this Gitea binary was not built with SQLite3 support, get an official release or rebuild with correct "-tags"`)
}

func registerSQLiteConnStrMaker(fn func(opts SQLiteConnStrOptions) (string, string, error)) {
	if slices.Contains(setting.SupportedDatabaseTypes, setting.DatabaseTypeSQLite3) {
		panic("another sqlite3 driver has been registered")
	}
	setting.SupportedDatabaseTypes = append(setting.SupportedDatabaseTypes, setting.DatabaseTypeSQLite3)
	makeSQLiteConnStr = fn
}

func ConnStrDefaultDatabase(opts ConnOptions) (string, string, error) {
	opts.Database, opts.Schema = "", ""
	return ConnStr(opts)
}

func ConnStr(opts ConnOptions) (string, string, error) {
	switch {
	case opts.Type.IsMySQL():
		// use unix socket or tcp socket
		connType := util.Iif(strings.HasPrefix(opts.Host, "/"), "unix", "tcp")
		// allow (Postgres-inspired) default value to work in MySQL
		tls := util.Iif(opts.SSLMode == "disable", "false", opts.SSLMode)
		// in case the database name is a partial connection string which contains "?" parameters
		paramSep := util.Iif(strings.Contains(opts.Database, "?"), "&", "?")
		connStr := fmt.Sprintf("%s:%s@%s(%s)/%s%sparseTime=true&tls=%s", opts.User, opts.Passwd, connType, opts.Host, opts.Database, paramSep, tls)
		return "mysql", connStr, nil

	case opts.Type.IsPostgreSQL():
		connStr := makePgSQLConnStr(opts.Host, opts.User, opts.Passwd, opts.Database, opts.SSLMode)
		driver := util.Iif(opts.Schema == "", "postgres", sqlDriverPostgresSchema)
		registerPostgresSchemaDriver()
		return driver, connStr, nil

	case opts.Type.IsMSSQL():
		host, port := parseMSSQLHostPort(opts.Host)
		connStr := fmt.Sprintf("server=%s; port=%s; user id=%s; password=%s;", host, port, opts.User, opts.Passwd)
		if opts.Database != "" {
			connStr += "; database=" + opts.Database
		}
		return "mssql", connStr, nil

	case opts.Type.IsSQLite3():
		if opts.SQLitePath == "" {
			return "", "", errors.New("sqlite3 database path cannot be empty")
		}
		if err := os.MkdirAll(filepath.Dir(opts.SQLitePath), os.ModePerm); err != nil {
			return "", "", fmt.Errorf("failed to create directories: %w", err)
		}
		return makeSQLiteConnStr(SQLiteConnStrOptions{
			FilePath:    opts.SQLitePath,
			JournalMode: opts.SQLiteJournalMode,
			BusyTimeout: opts.SQLiteBusyTimeout,
		})
	}
	return "", "", fmt.Errorf("unknown database type: %s", opts.Type)
}

// parsePgSQLHostPort parses given input in various forms defined in
// https://www.postgresql.org/docs/current/static/libpq-connect.html#LIBPQ-CONNSTRING
// and returns proper host and port number.
func parsePgSQLHostPort(info string) (host, port string) {
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

func makePgSQLConnStr(dbHost, dbUser, dbPasswd, dbName, dbsslMode string) (connStr string) {
	dbName, dbParam, _ := strings.Cut(dbName, "?")
	host, port := parsePgSQLHostPort(dbHost)
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

// parseMSSQLHostPort splits the host into host and port
func parseMSSQLHostPort(info string) (string, string) {
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
