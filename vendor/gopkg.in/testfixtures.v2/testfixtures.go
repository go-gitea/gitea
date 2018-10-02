package testfixtures

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// Context holds the fixtures to be loaded in the database.
type Context struct {
	db            *sql.DB
	helper        Helper
	fixturesFiles []*fixtureFile
}

type fixtureFile struct {
	path       string
	fileName   string
	content    []byte
	insertSQLs []insertSQL
}

type insertSQL struct {
	sql    string
	params []interface{}
}

var (
	dbnameRegexp = regexp.MustCompile("(?i)test")
)

// NewFolder creates a context for all fixtures in a given folder into the database:
//     NewFolder(db, &PostgreSQL{}, "my/fixtures/folder")
func NewFolder(db *sql.DB, helper Helper, folderName string) (*Context, error) {
	fixtures, err := fixturesFromFolder(folderName)
	if err != nil {
		return nil, err
	}

	c, err := newContext(db, helper, fixtures)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// NewFiles creates a context for all specified fixtures files into database:
//     NewFiles(db, &PostgreSQL{},
//         "fixtures/customers.yml",
//         "fixtures/orders.yml"
//         // add as many files you want
//     )
func NewFiles(db *sql.DB, helper Helper, fileNames ...string) (*Context, error) {
	fixtures, err := fixturesFromFiles(fileNames...)
	if err != nil {
		return nil, err
	}

	c, err := newContext(db, helper, fixtures)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func newContext(db *sql.DB, helper Helper, fixtures []*fixtureFile) (*Context, error) {
	c := &Context{
		db:            db,
		helper:        helper,
		fixturesFiles: fixtures,
	}

	if err := c.helper.init(c.db); err != nil {
		return nil, err
	}

	if err := c.buildInsertSQLs(); err != nil {
		return nil, err
	}

	return c, nil
}

// DetectTestDatabase returns nil if databaseName matches regexp
//     if err := fixtures.DetectTestDatabase(); err != nil {
//         log.Fatal(err)
//     }
func (c *Context) DetectTestDatabase() error {
	dbName, err := c.helper.databaseName(c.db)
	if err != nil {
		return err
	}
	if !dbnameRegexp.MatchString(dbName) {
		return ErrNotTestDatabase
	}
	return nil
}

// Load wipes and after load all fixtures in the database.
//     if err := fixtures.Load(); err != nil {
//         log.Fatal(err)
//     }
func (c *Context) Load() error {
	if !skipDatabaseNameCheck {
		if err := c.DetectTestDatabase(); err != nil {
			return err
		}
	}

	err := c.helper.disableReferentialIntegrity(c.db, func(tx *sql.Tx) error {
		for _, file := range c.fixturesFiles {
			modified, err := c.helper.isTableModified(tx, file.fileNameWithoutExtension())
			if err != nil {
				return err
			}
			if !modified {
				continue
			}
			if err := file.delete(tx, c.helper); err != nil {
				return err
			}

			err = c.helper.whileInsertOnTable(tx, file.fileNameWithoutExtension(), func() error {
				for j, i := range file.insertSQLs {
					if _, err := tx.Exec(i.sql, i.params...); err != nil {
						return &InsertError{
							Err:    err,
							File:   file.fileName,
							Index:  j,
							SQL:    i.sql,
							Params: i.params,
						}
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.helper.afterLoad(c.db)
}

func (c *Context) buildInsertSQLs() error {
	for _, f := range c.fixturesFiles {
		var records interface{}
		if err := yaml.Unmarshal(f.content, &records); err != nil {
			return err
		}

		switch records := records.(type) {
		case []interface{}:
			for _, record := range records {
				recordMap, ok := record.(map[interface{}]interface{})
				if !ok {
					return ErrWrongCastNotAMap
				}

				sql, values, err := f.buildInsertSQL(c.helper, recordMap)
				if err != nil {
					return err
				}

				f.insertSQLs = append(f.insertSQLs, insertSQL{sql, values})
			}
		case map[interface{}]interface{}:
			for _, record := range records {
				recordMap, ok := record.(map[interface{}]interface{})
				if !ok {
					return ErrWrongCastNotAMap
				}

				sql, values, err := f.buildInsertSQL(c.helper, recordMap)
				if err != nil {
					return err
				}

				f.insertSQLs = append(f.insertSQLs, insertSQL{sql, values})
			}
		default:
			return ErrFileIsNotSliceOrMap
		}
	}

	return nil
}

func (f *fixtureFile) fileNameWithoutExtension() string {
	return strings.Replace(f.fileName, filepath.Ext(f.fileName), "", 1)
}

func (f *fixtureFile) delete(tx *sql.Tx, h Helper) error {
	_, err := tx.Exec(fmt.Sprintf("DELETE FROM %s", h.quoteKeyword(f.fileNameWithoutExtension())))
	return err
}

func (f *fixtureFile) buildInsertSQL(h Helper, record map[interface{}]interface{}) (sqlStr string, values []interface{}, err error) {
	var (
		sqlColumns []string
		sqlValues  []string
		i          = 1
	)
	for key, value := range record {
		keyStr, ok := key.(string)
		if !ok {
			err = ErrKeyIsNotString
			return
		}

		sqlColumns = append(sqlColumns, h.quoteKeyword(keyStr))

		// if string, try convert to SQL or time
		// if map or array, convert to json
		switch v := value.(type) {
		case string:
			if strings.HasPrefix(v, "RAW=") {
				sqlValues = append(sqlValues, strings.TrimPrefix(v, "RAW="))
				continue
			}

			if t, err := tryStrToDate(v); err == nil {
				value = t
			}
		case []interface{}, map[interface{}]interface{}:
			value = recursiveToJSON(v)
		}

		switch h.paramType() {
		case paramTypeDollar:
			sqlValues = append(sqlValues, fmt.Sprintf("$%d", i))
		case paramTypeQuestion:
			sqlValues = append(sqlValues, "?")
		case paramTypeColon:
			sqlValues = append(sqlValues, fmt.Sprintf(":%d", i))
		}

		values = append(values, value)
		i++
	}

	sqlStr = fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		h.quoteKeyword(f.fileNameWithoutExtension()),
		strings.Join(sqlColumns, ", "),
		strings.Join(sqlValues, ", "),
	)
	return
}

func fixturesFromFolder(folderName string) ([]*fixtureFile, error) {
	var files []*fixtureFile
	fileinfos, err := ioutil.ReadDir(folderName)
	if err != nil {
		return nil, err
	}

	for _, fileinfo := range fileinfos {
		if !fileinfo.IsDir() && filepath.Ext(fileinfo.Name()) == ".yml" {
			fixture := &fixtureFile{
				path:     path.Join(folderName, fileinfo.Name()),
				fileName: fileinfo.Name(),
			}
			fixture.content, err = ioutil.ReadFile(fixture.path)
			if err != nil {
				return nil, err
			}
			files = append(files, fixture)
		}
	}
	return files, nil
}

func fixturesFromFiles(fileNames ...string) ([]*fixtureFile, error) {
	var (
		fixtureFiles []*fixtureFile
		err          error
	)

	for _, f := range fileNames {
		fixture := &fixtureFile{
			path:     f,
			fileName: filepath.Base(f),
		}
		fixture.content, err = ioutil.ReadFile(fixture.path)
		if err != nil {
			return nil, err
		}
		fixtureFiles = append(fixtureFiles, fixture)
	}

	return fixtureFiles, nil
}
