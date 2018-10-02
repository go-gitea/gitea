package testfixtures

import (
	"database/sql"
	"fmt"
	"strings"
)

// Oracle is the Oracle database helper for this package
type Oracle struct {
	baseHelper

	enabledConstraints []oracleConstraint
	sequences          []string
}

type oracleConstraint struct {
	tableName      string
	constraintName string
}

func (h *Oracle) init(db *sql.DB) error {
	var err error

	h.enabledConstraints, err = h.getEnabledConstraints(db)
	if err != nil {
		return err
	}

	h.sequences, err = h.getSequences(db)
	if err != nil {
		return err
	}

	return nil
}

func (*Oracle) paramType() int {
	return paramTypeColon
}

func (*Oracle) quoteKeyword(str string) string {
	return fmt.Sprintf("\"%s\"", strings.ToUpper(str))
}

func (*Oracle) databaseName(q queryable) (string, error) {
	var dbName string
	err := q.QueryRow("SELECT user FROM DUAL").Scan(&dbName)
	return dbName, err
}

func (*Oracle) tableNames(q queryable) ([]string, error) {
	query := `
		SELECT TABLE_NAME
		FROM USER_TABLES
	`
	rows, err := q.Query(query)
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

func (*Oracle) getEnabledConstraints(q queryable) ([]oracleConstraint, error) {
	var constraints []oracleConstraint
	rows, err := q.Query(`
		SELECT table_name, constraint_name
		FROM user_constraints
		WHERE constraint_type = 'R'
		  AND status = 'ENABLED'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var constraint oracleConstraint
		rows.Scan(&constraint.tableName, &constraint.constraintName)
		constraints = append(constraints, constraint)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return constraints, nil
}

func (*Oracle) getSequences(q queryable) ([]string, error) {
	var sequences []string
	rows, err := q.Query("SELECT sequence_name FROM user_sequences")
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
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return sequences, nil
}

func (h *Oracle) resetSequences(q queryable) error {
	for _, sequence := range h.sequences {
		_, err := q.Exec(fmt.Sprintf("DROP SEQUENCE %s", h.quoteKeyword(sequence)))
		if err != nil {
			return err
		}
		_, err = q.Exec(fmt.Sprintf("CREATE SEQUENCE %s START WITH %d", h.quoteKeyword(sequence), resetSequencesTo))
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Oracle) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) (err error) {
	// re-enable after load
	defer func() {
		for _, c := range h.enabledConstraints {
			_, err2 := db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE CONSTRAINT %s", h.quoteKeyword(c.tableName), h.quoteKeyword(c.constraintName)))
			if err2 != nil && err == nil {
				err = err2
			}
		}
	}()

	// disable foreign keys
	for _, c := range h.enabledConstraints {
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s DISABLE CONSTRAINT %s", h.quoteKeyword(c.tableName), h.quoteKeyword(c.constraintName)))
		if err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = loadFn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return h.resetSequences(db)
}
