package testfixtures

import (
	"database/sql"
)

type (
	DataBaseHelper Helper // Deprecated: Use Helper instead

	PostgreSQLHelper struct { // Deprecated: Use PostgreSQL{} instead
		PostgreSQL
		UseAlterConstraint bool
	}
	MySQLHelper struct { // Deprecated: Use MySQL{} instead
		MySQL
	}
	SQLiteHelper struct { // Deprecated: Use SQLite{} instead
		SQLite
	}
	SQLServerHelper struct { // Deprecated: Use SQLServer{} instead
		SQLServer
	}
	OracleHelper struct { // Deprecated: Use Oracle{} instead
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
