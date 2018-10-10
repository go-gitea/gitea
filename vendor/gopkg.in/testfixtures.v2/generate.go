package testfixtures

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"unicode/utf8"

	"gopkg.in/yaml.v2"
)

// TableInfo is settings for generating a fixture for table.
type TableInfo struct {
	Name  string // Table name
	Where string // A condition for extracting records. If this value is empty, extracts all records.
}

func (ti *TableInfo) whereClause() string {
	if ti.Where == "" {
		return ""
	}
	return fmt.Sprintf(" WHERE %s", ti.Where)
}

// GenerateFixtures generates fixtures for the current contents of a database, and saves
// them to the specified directory
func GenerateFixtures(db *sql.DB, helper Helper, dir string) error {
	tables, err := helper.tableNames(db)
	if err != nil {
		return err
	}
	for _, table := range tables {
		filename := path.Join(dir, table+".yml")
		if err := generateFixturesForTable(db, helper, &TableInfo{Name: table}, filename); err != nil {
			return err
		}
	}
	return nil
}

// GenerateFixturesForTables generates fixtures for the current contents of specified tables in a database, and saves
// them to the specified directory
func GenerateFixturesForTables(db *sql.DB, tables []*TableInfo, helper Helper, dir string) error {
	for _, table := range tables {
		filename := path.Join(dir, table.Name+".yml")
		if err := generateFixturesForTable(db, helper, table, filename); err != nil {
			return err
		}
	}
	return nil
}

func generateFixturesForTable(db *sql.DB, h Helper, table *TableInfo, filename string) error {
	query := fmt.Sprintf("SELECT * FROM %s%s", h.quoteKeyword(table.Name), table.whereClause())
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	fixtures := make([]interface{}, 0, 10)
	for rows.Next() {
		entries := make([]interface{}, len(columns))
		entryPtrs := make([]interface{}, len(entries))
		for i := range entries {
			entryPtrs[i] = &entries[i]
		}
		if err := rows.Scan(entryPtrs...); err != nil {
			return err
		}

		entryMap := make(map[string]interface{}, len(entries))
		for i, column := range columns {
			entryMap[column] = convertValue(entries[i])
		}
		fixtures = append(fixtures, entryMap)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	marshaled, err := yaml.Marshal(fixtures)
	if err != nil {
		return err
	}
	_, err = f.Write(marshaled)
	return err
}

func convertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
	}
	return value
}
