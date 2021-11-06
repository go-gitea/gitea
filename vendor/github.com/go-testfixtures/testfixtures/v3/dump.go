package testfixtures

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"gopkg.in/yaml.v2"
)

// Dumper is resposible for dumping fixtures from the database into a
// directory.
type Dumper struct {
	db     *sql.DB
	helper helper
	dir    string

	tables []string
}

// NewDumper creates a new dumper with the given options.
//
// The "DumpDatabase", "DumpDialect" and "DumpDirectory" options are required.
func NewDumper(options ...func(*Dumper) error) (*Dumper, error) {
	d := &Dumper{}

	for _, option := range options {
		if err := option(d); err != nil {
			return nil, err
		}
	}

	return d, nil
}

// DumpDatabase sets the database to be dumped.
func DumpDatabase(db *sql.DB) func(*Dumper) error {
	return func(d *Dumper) error {
		d.db = db
		return nil
	}
}

// DumpDialect informs Loader about which database dialect you're using.
//
// Possible options are "postgresql", "timescaledb", "mysql", "mariadb",
// "sqlite" and "sqlserver".
func DumpDialect(dialect string) func(*Dumper) error {
	return func(d *Dumper) error {
		h, err := helperForDialect(dialect)
		if err != nil {
			return err
		}
		d.helper = h
		return nil
	}
}

// DumpDirectory sets the directory where the fixtures files will be created.
func DumpDirectory(dir string) func(*Dumper) error {
	return func(d *Dumper) error {
		d.dir = dir
		return nil
	}
}

// DumpTables allows you to choose which tables you want to dump.
//
// If not informed, Dumper will dump all tables by default.
func DumpTables(tables ...string) func(*Dumper) error {
	return func(d *Dumper) error {
		d.tables = tables
		return nil
	}
}

// Dump dumps the databases as YAML fixtures.
func (d *Dumper) Dump() error {
	tables := d.tables
	if len(tables) == 0 {
		var err error
		tables, err = d.helper.tableNames(d.db)
		if err != nil {
			return err
		}
	}

	for _, table := range tables {
		if err := d.dumpTable(table); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dumper) dumpTable(table string) error {
	query := fmt.Sprintf("SELECT * FROM %s", d.helper.quoteKeyword(table))

	stmt, err := d.db.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	fixtures := make([]yaml.MapSlice, 0, 10)
	for rows.Next() {
		entries := make([]interface{}, len(columns))
		entryPtrs := make([]interface{}, len(entries))
		for i := range entries {
			entryPtrs[i] = &entries[i]
		}
		if err := rows.Scan(entryPtrs...); err != nil {
			return err
		}

		entryMap := make([]yaml.MapItem, len(entries))
		for i, column := range columns {
			entryMap[i] = yaml.MapItem{
				Key:   column,
				Value: convertValue(entries[i]),
			}
		}
		fixtures = append(fixtures, entryMap)
	}
	if err = rows.Err(); err != nil {
		return err
	}

	filePath := filepath.Join(d.dir, table+".yml")
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := yaml.Marshal(fixtures)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

func convertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return "0x" + hex.EncodeToString(value.([]byte))
	}
	return value
}
