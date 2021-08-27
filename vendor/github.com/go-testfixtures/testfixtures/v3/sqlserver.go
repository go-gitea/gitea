package testfixtures

import (
	"database/sql"
	"fmt"
	"strings"
)

type sqlserver struct {
	baseHelper

	paramTypeCache int
	tables         []string
}

func (h *sqlserver) init(db *sql.DB) error {
	var err error

	// NOTE(@andreynering): The SQL Server lib (github.com/denisenkom/go-mssqldb)
	// supports both the "?" style (when using the deprecated "mssql" driver)
	// and the "@p1" style (when using the new "sqlserver" driver).
	//
	// Since we don't have a way to know which driver it's been used,
	// this is a small hack to detect the allowed param style.
	var v int
	if err := db.QueryRow("SELECT ?", 1).Scan(&v); err == nil && v == 1 {
		h.paramTypeCache = paramTypeQuestion
	} else {
		h.paramTypeCache = paramTypeAtSign
	}

	h.tables, err = h.tableNames(db)
	if err != nil {
		return err
	}

	return nil
}

func (h *sqlserver) paramType() int {
	return h.paramTypeCache
}

func (*sqlserver) quoteKeyword(s string) string {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		parts[i] = fmt.Sprintf(`[%s]`, p)
	}
	return strings.Join(parts, ".")
}

func (*sqlserver) databaseName(q queryable) (string, error) {
	var dbName string
	err := q.QueryRow("SELECT DB_NAME()").Scan(&dbName)
	return dbName, err
}

func (*sqlserver) tableNames(q queryable) ([]string, error) {
	rows, err := q.Query("SELECT table_schema + '.' + table_name FROM information_schema.tables WHERE table_name <> 'spt_values' AND table_type = 'BASE TABLE'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

func (h *sqlserver) tableHasIdentityColumn(q queryable, tableName string) (bool, error) {
	sql := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM SYS.IDENTITY_COLUMNS
		WHERE OBJECT_ID = OBJECT_ID('%s')
	`, tableName)
	var count int
	if err := q.QueryRow(sql).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil

}

func (h *sqlserver) whileInsertOnTable(tx *sql.Tx, tableName string, fn func() error) (err error) {
	hasIdentityColumn, err := h.tableHasIdentityColumn(tx, tableName)
	if err != nil {
		return err
	}
	if hasIdentityColumn {
		defer func() {
			_, err2 := tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s OFF", h.quoteKeyword(tableName)))
			if err2 != nil && err == nil {
				err = fmt.Errorf("testfixtures: could not disable identity insert: %w", err2)
			}
		}()

		_, err := tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s ON", h.quoteKeyword(tableName)))
		if err != nil {
			return fmt.Errorf("testfixtures: could not enable identity insert: %w", err)
		}
	}
	return fn()
}

func (h *sqlserver) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) (err error) {
	// ensure the triggers are re-enable after all
	defer func() {
		var b strings.Builder
		for _, table := range h.tables {
			b.WriteString(fmt.Sprintf("ALTER TABLE %s WITH CHECK CHECK CONSTRAINT ALL;", h.quoteKeyword(table)))
		}
		if _, err2 := db.Exec(b.String()); err2 != nil && err == nil {
			err = err2
		}
	}()

	var b strings.Builder
	for _, table := range h.tables {
		b.WriteString(fmt.Sprintf("ALTER TABLE %s NOCHECK CONSTRAINT ALL;", h.quoteKeyword(table)))
	}
	if _, err := db.Exec(b.String()); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = loadFn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// splitter is a batchSplitter interface implementation. We need it for
// SQL Server because commands like a `CREATE SCHEMA...` and a `CREATE TABLE...`
// could not be executed in the same batch.
// See https://docs.microsoft.com/en-us/previous-versions/sql/sql-server-2008-r2/ms175502(v=sql.105)#rules-for-using-batches
func (*sqlserver) splitter() []byte {
	return []byte("GO\n")
}
