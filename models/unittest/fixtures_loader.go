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

	"code.gitea.io/gitea/models/db"

	"gopkg.in/yaml.v3"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type FixtureItem struct {
	fileFullPath string
	tableName    string

	tableNameQuoted string
	sqlInserts      []string
	sqlInsertArgs   [][]any

	mssqlHasIdentityColumn bool
}

type fixturesLoaderInternal struct {
	xormEngine       *xorm.Engine
	xormTableNames   map[string]bool
	db               *sql.DB
	dbType           schemas.DBType
	fixtures         map[string]*FixtureItem
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

func (f *fixturesLoaderInternal) prepareFixtureItem(fixture *FixtureItem) (err error) {
	fixture.tableNameQuoted = f.quoteObject(fixture.tableName)

	if f.dbType == schemas.MSSQL {
		fixture.mssqlHasIdentityColumn, err = f.mssqlTableHasIdentityColumn(f.db, fixture.tableName)
		if err != nil {
			return err
		}
	}

	data, err := os.ReadFile(fixture.fileFullPath)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", fixture.fileFullPath, err)
	}

	var rows []map[string]any
	if err = yaml.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("failed to unmarshal yaml data from %q: %w", fixture.fileFullPath, err)
	}
	if err = f.preprocessFixtureRow(rows); err != nil {
		return fmt.Errorf("failed to preprocess fixture rows from %q: %w", fixture.fileFullPath, err)
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
	return nil
}

func (f *fixturesLoaderInternal) loadFixtures(tx *sql.Tx, fixture *FixtureItem) (err error) {
	if fixture.tableNameQuoted == "" {
		if err = f.prepareFixtureItem(fixture); err != nil {
			return err
		}
	}

	_, err = tx.Exec("DELETE FROM " + fixture.tableNameQuoted) // sqlite3 doesn't support truncate
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

	for _, fixture := range f.fixtures {
		if !f.xormTableNames[fixture.tableName] {
			continue
		}
		if err := f.loadFixtures(tx, fixture); err != nil {
			return fmt.Errorf("failed to load fixtures from %s: %w", fixture.fileFullPath, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for xormTableName := range f.xormTableNames {
		if f.fixtures[xormTableName] == nil {
			_, _ = f.xormEngine.Exec("DELETE FROM `" + xormTableName + "`")
		}
	}
	return nil
}

func FixturesFileFullPaths(dir string, files []string) (map[string]*FixtureItem, error) {
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
	fixtureItems := map[string]*FixtureItem{}
	for _, file := range files {
		fileFillPath := file
		if !filepath.IsAbs(fileFillPath) {
			fileFillPath = filepath.Join(dir, file)
		}
		tableName, _, _ := strings.Cut(filepath.Base(file), ".")
		fixtureItems[tableName] = &FixtureItem{fileFullPath: fileFillPath, tableName: tableName}
	}
	return fixtureItems, nil
}

func NewFixturesLoader(x *xorm.Engine, opts FixturesOptions) (FixturesLoader, error) {
	fixtureItems, err := FixturesFileFullPaths(opts.Dir, opts.Files)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures files: %w", err)
	}

	f := &fixturesLoaderInternal{xormEngine: x, db: x.DB().DB, dbType: x.Dialect().URI().DBType, fixtures: fixtureItems}
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

	xormBeans, _ := db.NamesToBean()
	f.xormTableNames = map[string]bool{}
	for _, bean := range xormBeans {
		f.xormTableNames[x.TableName(bean)] = true
	}

	return f, nil
}
