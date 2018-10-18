package testfixtures

import (
	"database/sql"
)

type (
	// DataBaseHelper is the helper interface
	// Deprecated: Use Helper instead
	DataBaseHelper Helper

	// PostgreSQLHelper is the PostgreSQL helper
	// Deprecated: Use PostgreSQL{} instead
	PostgreSQLHelper struct {
		PostgreSQL
		UseAlterConstraint bool
	}

	// MySQLHelper is the MySQL helper
	// Deprecated: Use MySQL{} instead
	MySQLHelper struct {
		MySQL
	}

	// SQLiteHelper is the SQLite helper
	// Deprecated: Use SQLite{} instead
	SQLiteHelper struct {
		SQLite
	}

	// SQLServerHelper is the SQLServer helper
	// Deprecated: Use SQLServer{} instead
	SQLServerHelper struct {
		SQLServer
	}

	// OracleHelper is the Oracle helper
	// Deprecated: Use Oracle{} instead
	OracleHelper struct {
		Oracle
	}
)

func (h *PostgreSQLHelper) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) error {
	h.PostgreSQL.UseAlterConstraint = h.UseAlterConstraint
	return h.PostgreSQL.disableReferentialIntegrity(db, loadFn)
}

// LoadFixtureFiles load all specified fixtures files into database:
// 		LoadFixtureFiles(db, &PostgreSQL{},
// 			"fixtures/customers.yml", "fixtures/orders.yml")
//			// add as many files you want
//
// Deprecated: Use NewFiles() and Load() instead.
func LoadFixtureFiles(db *sql.DB, helper Helper, files ...string) error {
	c, err := NewFiles(db, helper, files...)
	if err != nil {
		return err
	}

	return c.Load()
}

// LoadFixtures loads all fixtures in a given folder into the database:
// 		LoadFixtures("myfixturesfolder", db, &PostgreSQL{})
//
// Deprecated: Use NewFolder() and Load() instead.
func LoadFixtures(folderName string, db *sql.DB, helper Helper) error {
	c, err := NewFolder(db, helper, folderName)
	if err != nil {
		return err
	}

	return c.Load()
}
