// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gitea.dev/models/db"

	"go.yaml.in/yaml/v4"
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
	tableSyncMap     sync.Map
	db               *sql.DB
	dbType           schemas.DBType
	fixtures         map[string]*FixtureItem
	quoteObject      func(string) string
	paramPlaceholder func(idx int) string
	boolColumns      map[string]map[string]struct{}
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

// coerceBoolArgs rewrites the 0/1 spelling several fixtures use for booleans into
// real bools. The Firebird driver matches the parameter type against the column and
// has no plan to encode an int into a boolean, unlike the other supported databases.
func (f *fixturesLoaderInternal) coerceBoolArgs(tableName string, rows []map[string]any) {
	boolColumns := f.boolColumns[tableName]
	if len(boolColumns) == 0 {
		return
	}
	for _, row := range rows {
		for k, v := range row {
			n, ok := v.(int)
			if !ok {
				continue
			}
			if _, isBool := boolColumns[k]; isBool {
				row[k] = n != 0
			}
		}
	}
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
	f.coerceBoolArgs(fixture.tableName, rows)

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
	// A finished test's background work can still write rows this load replaces, the
	// two transactions deadlock and Firebird's monitor kills the loader side
	// (SQLSTATE 40001). The load is idempotent (delete all, insert), so a retry lets
	// the background drain; on the other databases the conflict error never surfaces.
	for range 3 {
		err := f.load()
		if err == nil || !db.IsConflictError(err) {
			return err
		}
		time.Sleep(time.Second)
	}
	return f.load()
}

func (f *fixturesLoaderInternal) load() error {
	tx, err := f.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ctx := context.WithValue(context.Background(), db.ContextKeyTestFixtures, true)

	for _, fixture := range f.fixtures {
		synced, existing := f.tableSyncMap.Load(f.syncTableKey(fixture.tableName))
		if synced == true || !existing {
			continue
		}
		if err := f.loadFixtures(tx, fixture); err != nil {
			return fmt.Errorf("failed to load fixtures from %s: %w", fixture.fileFullPath, err)
		}
		f.tableSyncMap.Store(f.syncTableKey(fixture.tableName), true)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	f.tableSyncMap.Range(func(k, v any) bool {
		tableName, _ := k.(string)
		synced, _ := v.(bool)
		if !synced && !f.hasFixture(tableName) {
			_, _ = f.xormEngine.Context(ctx).Exec("DELETE FROM " + f.quoteObject(tableName))
		}
		f.tableSyncMap.Store(tableName, true)
		return true
	})
	return nil
}

// syncTableKey normalizes a table name to the key it is tracked by in
// tableSyncMap. Firebird folds unquoted identifiers to upper case, so the SQL a
// test runs names tables in upper case while the fixture files and the bean names
// are lower case, and the table would never be marked as changed.
func (f *fixturesLoaderInternal) syncTableKey(tableName string) string {
	if f.dbType == schemas.FIREBIRD {
		return strings.ToUpper(tableName)
	}
	return tableName
}

func (f *fixturesLoaderInternal) hasFixture(tableName string) bool {
	for name := range f.fixtures {
		if f.syncTableKey(name) == tableName {
			return true
		}
	}
	return false
}

func (f *fixturesLoaderInternal) MarkTableChanged(tableName string) {
	f.tableSyncMap.Store(f.syncTableKey(tableName), false)
}

func FixturesFileFullPaths(dir string, files []string) (map[string]*FixtureItem, error) {
	if files != nil && len(files) == 0 {
		return nil, nil //nolint:nilnil // load nothing
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
	if f.dbType == schemas.FIREBIRD {
		f.boolColumns = make(map[string]map[string]struct{})
	}
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
	case schemas.FIREBIRD:
		// the Firebird dialect creates objects unquoted, which Firebird folds to upper
		// case, so the quoted form has to match that or the lookup misses
		f.quoteObject = func(s string) string { return fmt.Sprintf(`"%s"`, strings.ToUpper(s)) }
		f.paramPlaceholder = func(idx int) string { return "?" }
	}

	// If a model is not imported in a package (no bean is registered), the table won't exist in database.
	// So only use tables of registered models (beans).
	xormBeans, _ := db.NamesToBean()
	for _, bean := range xormBeans {
		beanTableName := x.TableName(bean)
		tableName := trimTableNameQuotes(beanTableName)
		f.tableSyncMap.Store(f.syncTableKey(tableName), false)
		if f.dbType != schemas.FIREBIRD {
			continue
		}
		table, err := x.TableInfo(bean)
		if err != nil {
			continue
		}
		boolColumns := make(map[string]struct{})
		for _, colName := range table.ColumnsSeq() {
			if col := table.GetColumn(colName); col.SQLType.IsBool() {
				boolColumns[col.Name] = struct{}{}
			}
		}
		f.boolColumns[tableName] = boolColumns
	}
	return f, nil
}
