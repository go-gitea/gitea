package testfixtures

var (
	skipDatabaseNameCheck bool
	resetSequencesTo      int64 = 10000
)

// SkipDatabaseNameCheck If true, loading fixtures will not check if the database
// name constaint "test". Use with caution!
func SkipDatabaseNameCheck(value bool) {
	skipDatabaseNameCheck = value
}

// ResetSequencesTo sets the value the sequences will be reset to.
// This is used by PostgreSQL and Oracle.
// Defaults to 10000.
func ResetSequencesTo(value int64) {
	resetSequencesTo = value
}
