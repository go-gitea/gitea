package testfixtures

import (
	"database/sql"
	"fmt"
)

// MySQL is the MySQL helper for this package
type MySQL struct {
	baseHelper
	tables         []string
	tablesChecksum map[string]int64
}

func (h *MySQL) init(db *sql.DB) error {
	var err error
	h.tables, err = h.tableNames(db)
	if err != nil {
		return err
	}

	return nil
}

func (*MySQL) paramType() int {
	return paramTypeQuestion
}

func (*MySQL) quoteKeyword(str string) string {
	return fmt.Sprintf("`%s`", str)
}

func (*MySQL) databaseName(q queryable) (string, error) {
	var dbName string
	err := q.QueryRow("SELECT DATABASE()").Scan(&dbName)
	return dbName, err
}

func (h *MySQL) tableNames(q queryable) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		  AND table_type = 'BASE TABLE';
	`
	dbName, err := h.databaseName(q)
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(query, dbName)
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

func (h *MySQL) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) (err error) {
	// re-enable after load
	defer func() {
		if _, err2 := db.Exec("SET FOREIGN_KEY_CHECKS = 1"); err2 != nil && err == nil {
			err = err2
		}
	}()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}

	if err = loadFn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (h *MySQL) isTableModified(q queryable, tableName string) (bool, error) {
	checksum, err := h.getChecksum(q, tableName)
	if err != nil {
		return true, err
	}

	oldChecksum := h.tablesChecksum[tableName]

	return oldChecksum == 0 || checksum != oldChecksum, nil
}

func (h *MySQL) afterLoad(q queryable) error {
	if h.tablesChecksum != nil {
		return nil
	}

	h.tablesChecksum = make(map[string]int64, len(h.tables))
	for _, t := range h.tables {
		checksum, err := h.getChecksum(q, t)
		if err != nil {
			return err
		}
		h.tablesChecksum[t] = checksum
	}
	return nil
}

func (h *MySQL) getChecksum(q queryable, tableName string) (int64, error) {
	sql := fmt.Sprintf("CHECKSUM TABLE %s", h.quoteKeyword(tableName))
	var (
		table    string
		checksum int64
	)
	if err := q.QueryRow(sql).Scan(&table, &checksum); err != nil {
		return 0, err
	}
	return checksum, nil
}
