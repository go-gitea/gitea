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

func (*Oracle) databaseName(db *sql.DB) (dbName string) {
	db.QueryRow("SELECT user FROM DUAL").Scan(&dbName)
	return
}

func (*Oracle) getEnabledConstraints(db *sql.DB) ([]oracleConstraint, error) {
	constraints := make([]oracleConstraint, 0)
	rows, err := db.Query(`
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
	return constraints, nil
}

func (*Oracle) getSequences(db *sql.DB) ([]string, error) {
	sequences := make([]string, 0)
	rows, err := db.Query("SELECT sequence_name FROM user_sequences")
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var sequence string
		rows.Scan(&sequence)
		sequences = append(sequences, sequence)
	}
	return sequences, nil
}

func (h *Oracle) resetSequences(db *sql.DB) error {
	for _, sequence := range h.sequences {
		_, err := db.Exec(fmt.Sprintf("DROP SEQUENCE %s", h.quoteKeyword(sequence)))
		if err != nil {
			return err
		}
		_, err = db.Exec(fmt.Sprintf("CREATE SEQUENCE %s START WITH %d", h.quoteKeyword(sequence), resetSequencesTo))
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Oracle) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) error {
	// re-enable after load
	defer func() {
		for _, c := range h.enabledConstraints {
			db.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE CONSTRAINT %s", h.quoteKeyword(c.tableName), h.quoteKeyword(c.constraintName)))
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

	if err = loadFn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return h.resetSequences(db)
}
