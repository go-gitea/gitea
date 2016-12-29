package testfixtures

import (
	"database/sql"
	"fmt"
)

// PostgreSQL is the PG helper for this package
type PostgreSQL struct {
	baseHelper

	// UseAlterConstraint If true, the contraint disabling will do
	// using ALTER CONTRAINT sintax, only allowed in PG >= 9.4.
	// If false, the constraint disabling will use DISABLE TRIGGER ALL,
	// which requires SUPERUSER privileges.
	UseAlterConstraint bool

	tables                   []string
	sequences                []string
	nonDeferrableConstraints []pgConstraint
}

type pgConstraint struct {
	tableName      string
	constraintName string
}

func (h *PostgreSQL) init(db *sql.DB) error {
	var err error

	h.tables, err = h.getTables(db)
	if err != nil {
		return err
	}

	h.sequences, err = h.getSequences(db)
	if err != nil {
		return err
	}

	h.nonDeferrableConstraints, err = h.getNonDeferrableConstraints(db)
	if err != nil {
		return err
	}

	return nil
}

func (*PostgreSQL) paramType() int {
	return paramTypeDollar
}

func (*PostgreSQL) databaseName(db *sql.DB) (dbName string) {
	db.QueryRow("SELECT current_database()").Scan(&dbName)
	return
}

func (h *PostgreSQL) getTables(db *sql.DB) ([]string, error) {
	var tables []string

	sql := `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_type = 'BASE TABLE';
`
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}
	return tables, nil
}

func (h *PostgreSQL) getSequences(db *sql.DB) ([]string, error) {
	var sequences []string

	sql := "SELECT relname FROM pg_class WHERE relkind = 'S'"
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var sequence string
		if err = rows.Scan(&sequence); err != nil {
			return nil, err
		}
		sequences = append(sequences, sequence)
	}
	return sequences, nil
}

func (*PostgreSQL) getNonDeferrableConstraints(db *sql.DB) ([]pgConstraint, error) {
	var constraints []pgConstraint

	sql := `
SELECT table_name, constraint_name
FROM information_schema.table_constraints
WHERE constraint_type = 'FOREIGN KEY'
  AND is_deferrable = 'NO'`
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var constraint pgConstraint
		err = rows.Scan(&constraint.tableName, &constraint.constraintName)
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, constraint)
	}
	return constraints, nil
}

func (h *PostgreSQL) disableTriggers(db *sql.DB, loadFn loadFunction) error {
	defer func() {
		// re-enable triggers after load
		var sql string
		for _, table := range h.tables {
			sql += fmt.Sprintf("ALTER TABLE %s ENABLE TRIGGER ALL;", h.quoteKeyword(table))
		}
		db.Exec(sql)
	}()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var sql string
	for _, table := range h.tables {
		sql += fmt.Sprintf("ALTER TABLE %s DISABLE TRIGGER ALL;", h.quoteKeyword(table))
	}
	if _, err = tx.Exec(sql); err != nil {
		return err
	}

	if err = loadFn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (h *PostgreSQL) makeConstraintsDeferrable(db *sql.DB, loadFn loadFunction) error {
	defer func() {
		// ensure constraint being not deferrable again after load
		var sql string
		for _, constraint := range h.nonDeferrableConstraints {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER CONSTRAINT %s NOT DEFERRABLE;", h.quoteKeyword(constraint.tableName), h.quoteKeyword(constraint.constraintName))
		}
		db.Exec(sql)
	}()

	var sql string
	for _, constraint := range h.nonDeferrableConstraints {
		sql += fmt.Sprintf("ALTER TABLE %s ALTER CONSTRAINT %s DEFERRABLE;", h.quoteKeyword(constraint.tableName), h.quoteKeyword(constraint.constraintName))
	}
	if _, err := db.Exec(sql); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if _, err = tx.Exec("SET CONSTRAINTS ALL DEFERRED"); err != nil {
		return nil
	}

	if err = loadFn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (h *PostgreSQL) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) error {
	// ensure sequences being reset after load
	defer h.resetSequences(db)

	if h.UseAlterConstraint {
		return h.makeConstraintsDeferrable(db, loadFn)
	} else {
		return h.disableTriggers(db, loadFn)
	}
}

func (h *PostgreSQL) resetSequences(db *sql.DB) error {
	for _, sequence := range h.sequences {
		_, err := db.Exec(fmt.Sprintf("SELECT SETVAL('%s', %d)", sequence, resetSequencesTo))
		if err != nil {
			return err
		}
	}
	return nil
}
