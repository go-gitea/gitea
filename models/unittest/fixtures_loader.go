// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type fixtureItem struct {
	tableName       string
	tableNameQuoted string
	sqlInserts      []string
	sqlInsertArgs   [][]any

	mssqlHasIdentityColumn bool
}

type fixturesLoaderInternal struct {
	db               *sql.DB
	dbType           schemas.DBType
	files            []string
	fixtures         map[string]*fixtureItem
	quoteObject      func(string) string
	paramPlaceholder func(idx int) string
}

func (f *fixturesLoaderInternal) mssqlTableHasIdentityColumn(db *sql.DB, tableName string) (bool, error) {
	row := db.QueryRow(`SELECT COUNT(*) FROM sys.identity_columns WHERE OBJECT_ID = OBJECT_ID(?)`, tableName)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (f *fixturesLoaderInternal) preprocessFixtureRow(row []map[string]any) (err error) {
	for _, m := range row {
		for k, v := range m {
			if s, ok := v.(string); ok {
				if strings.HasPrefix(s, "0x") {
					if m[k], err = hex.DecodeString(s[2:]); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (f *fixturesLoaderInternal) prepareFixtureItem(file string) (_ *fixtureItem, err error) {
	fixture := &fixtureItem{}
	fixture.tableName, _, _ = strings.Cut(filepath.Base(file), ".")
	fixture.tableNameQuoted = f.quoteObject(fixture.tableName)

	if f.dbType == schemas.MSSQL {
		fixture.mssqlHasIdentityColumn, err = f.mssqlTableHasIdentityColumn(f.db, fixture.tableName)
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", file, err)
	}

	var rows []map[string]any
	if err = yaml.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml data from %q: %w", file, err)
	}
	if err = f.preprocessFixtureRow(rows); err != nil {
		return nil, fmt.Errorf("failed to preprocess fixture rows from %q: %w", file, err)
	}

	var sqlBuf []byte
	var sqlArguments []any
	for _, row := range rows {
		sqlBuf = append(sqlBuf, fmt.Sprintf("INSERT INTO %s (", fixture.tableNameQuoted)...)
		for k, v := range row {
			sqlBuf = append(sqlBuf, f.quoteObject(k)...)
			sqlBuf = append(sqlBuf, ","...)
			sqlArguments = append(sqlArguments, v)
		}
		sqlBuf = sqlBuf[:len(sqlBuf)-1]
		sqlBuf = append(sqlBuf, ") VALUES ("...)
		paramIdx := 1
		for range row {
			sqlBuf = append(sqlBuf, f.paramPlaceholder(paramIdx)...)
			sqlBuf = append(sqlBuf, ',')
			paramIdx++
		}
		sqlBuf[len(sqlBuf)-1] = ')'
		fixture.sqlInserts = append(fixture.sqlInserts, string(sqlBuf))
		fixture.sqlInsertArgs = append(fixture.sqlInsertArgs, slices.Clone(sqlArguments))
		sqlBuf = sqlBuf[:0]
		sqlArguments = sqlArguments[:0]
	}
	return fixture, nil
}

func (f *fixturesLoaderInternal) loadFixtures(tx *sql.Tx, file string) (err error) {
	fixture := f.fixtures[file]
	if fixture == nil {
		if fixture, err = f.prepareFixtureItem(file); err != nil {
			return err
		}
		f.fixtures[file] = fixture
	}

	_, err = tx.Exec(fmt.Sprintf("DELETE FROM %s", fixture.tableNameQuoted)) // sqlite3 doesn't support truncate
	if err != nil {
		return err
	}

	if fixture.mssqlHasIdentityColumn {
		_, err = tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s ON", fixture.tableNameQuoted))
		if err != nil {
			return err
		}
		defer func() { _, err = tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s OFF", fixture.tableNameQuoted)) }()
	}
	for i := range fixture.sqlInserts {
		_, err = tx.Exec(fixture.sqlInserts[i], fixture.sqlInsertArgs[i]...)
	}
	if err != nil {
		return err
	}
	return nil
}

func (f *fixturesLoaderInternal) Load() error {
	tx, err := f.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, file := range f.files {
		if err := f.loadFixtures(tx, file); err != nil {
			return fmt.Errorf("failed to load fixtures from %s: %w", file, err)
		}
	}
	return tx.Commit()
}

func FixturesFileFullPaths(dir string, files []string) ([]string, error) {
	if files != nil && len(files) == 0 {
		return nil, nil // load nothing
	}
	files = slices.Clone(files)
	if len(files) == 0 {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			files = append(files, e.Name())
		}
	}
	for i, file := range files {
		if !filepath.IsAbs(file) {
			files[i] = filepath.Join(dir, file)
		}
	}
	return files, nil
}

func NewFixturesLoader(x *xorm.Engine, opts FixturesOptions) (FixturesLoader, error) {
	files, err := FixturesFileFullPaths(opts.Dir, opts.Files)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures files: %w", err)
	}
	f := &fixturesLoaderInternal{db: x.DB().DB, dbType: x.Dialect().URI().DBType, files: files, fixtures: map[string]*fixtureItem{}}
	switch f.dbType {
	case schemas.SQLITE:
		f.quoteObject = func(s string) string { return fmt.Sprintf(`"%s"`, s) }
		f.paramPlaceholder = func(idx int) string { return "?" }
	case schemas.POSTGRES:
		f.quoteObject = func(s string) string { return fmt.Sprintf(`"%s"`, s) }
		f.paramPlaceholder = func(idx int) string { return fmt.Sprintf(`$%d`, idx) }
	case schemas.MYSQL:
		f.quoteObject = func(s string) string { return fmt.Sprintf("`%s`", s) }
		f.paramPlaceholder = func(idx int) string { return "?" }
	case schemas.MSSQL:
		f.quoteObject = func(s string) string { return fmt.Sprintf("[%s]", s) }
		f.paramPlaceholder = func(idx int) string { return "?" }
	}
	return f, nil
}
