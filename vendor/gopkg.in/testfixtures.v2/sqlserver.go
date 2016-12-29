package testfixtures

import (
	"database/sql"
	"fmt"
)

// SQLServer is the helper for SQL Server for this package.
// SQL Server >= 2008 is required.
type SQLServer struct {
	baseHelper

	tables []string
}

func (h *SQLServer) init(db *sql.DB) error {
	var err error

	h.tables, err = h.getTables(db)
	if err != nil {
		return err
	}

	return nil
}

func (*SQLServer) paramType() int {
	return paramTypeQuestion
}

func (*SQLServer) quoteKeyword(str string) string {
	return fmt.Sprintf("[%s]", str)
}

func (*SQLServer) databaseName(db *sql.DB) (dbname string) {
	db.QueryRow("SELECT DB_NAME()").Scan(&dbname)
	return
}

func (*SQLServer) getTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT table_name FROM information_schema.tables")
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0)
	defer rows.Close()
	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}
	return tables, nil
}

func (*SQLServer) tableHasIdentityColumn(tx *sql.Tx, tableName string) bool {
	sql := `
SELECT COUNT(*)
FROM SYS.IDENTITY_COLUMNS
WHERE OBJECT_NAME(OBJECT_ID) = ?
`
	var count int
	tx.QueryRow(sql, tableName).Scan(&count)
	return count > 0

}

func (h *SQLServer) whileInsertOnTable(tx *sql.Tx, tableName string, fn func() error) error {
	if h.tableHasIdentityColumn(tx, tableName) {
		defer tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s OFF", h.quoteKeyword(tableName)))
		_, err := tx.Exec(fmt.Sprintf("SET IDENTITY_INSERT %s ON", h.quoteKeyword(tableName)))
		if err != nil {
			return err
		}
	}
	return fn()
}

func (h *SQLServer) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) error {
	// ensure the triggers are re-enable after all
	defer func() {
		sql := ""
		for _, table := range h.tables {
			sql += fmt.Sprintf("ALTER TABLE %s WITH CHECK CHECK CONSTRAINT ALL;", h.quoteKeyword(table))
		}
		if _, err := db.Exec(sql); err != nil {
			fmt.Printf("Error on re-enabling constraints: %v\n", err)
		}
	}()

	sql := ""
	for _, table := range h.tables {
		sql += fmt.Sprintf("ALTER TABLE %s NOCHECK CONSTRAINT ALL;", h.quoteKeyword(table))
	}
	if _, err := db.Exec(sql); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err = loadFn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
