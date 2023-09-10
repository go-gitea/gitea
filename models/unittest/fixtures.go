// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package unittest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-testfixtures/testfixtures/v3"
	"xorm.io/xorm"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var fixturesLoader *testfixtures.Loader

// GetXORMEngine gets the XORM engine
func GetXORMEngine(engine ...*xorm.Engine) (x *xorm.Engine) {
	if len(engine) == 1 {
		return engine[0]
	}
	return db.DefaultContext.(*db.Context).Engine().(*xorm.Engine)
}

// InitFixtures initialize test fixtures for a test database
func InitFixtures(opts FixturesOptions, engine ...*xorm.Engine) (err error) {
	e := GetXORMEngine(engine...)
	var fixtureOptionFiles func(*testfixtures.Loader) error
	if opts.Dir != "" {
		fixtureOptionFiles = testfixtures.Directory(opts.Dir)
	} else {
		fixtureOptionFiles = testfixtures.Files(opts.Files...)
	}
	dialect := "unknown"
	switch e.Dialect().URI().DBType {
	case schemas.POSTGRES:
		dialect = "postgres"
	case schemas.MYSQL:
		dialect = "mysql"
	case schemas.MSSQL:
		dialect = "mssql"
	case schemas.SQLITE:
		dialect = "sqlite3"
	default:
		fmt.Println("Unsupported RDBMS for integration tests")
		os.Exit(1)
	}
	loaderOptions := []func(loader *testfixtures.Loader) error{
		testfixtures.Database(e.DB().DB),
		testfixtures.Dialect(dialect),
		testfixtures.DangerousSkipTestDatabaseCheck(),
		fixtureOptionFiles,
	}

	if e.Dialect().URI().DBType == schemas.POSTGRES {
		loaderOptions = append(loaderOptions, testfixtures.SkipResetSequences())
	}

	fixturesLoader, err = testfixtures.New(loaderOptions...)
	if err != nil {
		return err
	}

	// register the dummy hash algorithm function used in the test fixtures
	_ = hash.Register("dummy", hash.NewDummyHasher)

	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")

	return err
}

func DumpAllFixtures(dir string) error {
	return db.AllTablesForEach(func(info *schemas.Table, bean any) error {
		pth := path.Join(dir, info.Name+".yml")
		fd, err := os.OpenFile(pth, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer fd.Close()

		sum, err := db.GetEngine(db.DefaultContext).Count(bean)
		if err != nil {
			return err
		}
		if sum == 0 {
			_, err = fd.WriteString("[] # empty\n")
			return err
		}

		return db.GetEngine(db.DefaultContext).Iterate(bean, func(idx int, data interface{}) error {
			return DefaultFixtureDumper(data, fd)
		})
	})
}

// LoadFixtures load fixtures for a test database
func LoadFixtures(engine ...*xorm.Engine) error {
	e := GetXORMEngine(engine...)
	var err error
	// (doubt) database transaction conflicts could occur and result in ROLLBACK? just try for a few times.
	for i := 0; i < 5; i++ {
		if err = fixturesLoader.Load(); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		fmt.Printf("LoadFixtures failed after retries: %v\n", err)
	}
	// Now if we're running postgres we need to tell it to update the sequences
	if e.Dialect().URI().DBType == schemas.POSTGRES {
		results, err := e.QueryString(`SELECT 'SELECT SETVAL(' ||
		quote_literal(quote_ident(PGT.schemaname) || '.' || quote_ident(S.relname)) ||
		', COALESCE(MAX(' ||quote_ident(C.attname)|| '), 1) ) FROM ' ||
		quote_ident(PGT.schemaname)|| '.'||quote_ident(T.relname)|| ';'
	 FROM pg_class AS S,
	      pg_depend AS D,
	      pg_class AS T,
	      pg_attribute AS C,
	      pg_tables AS PGT
	 WHERE S.relkind = 'S'
	     AND S.oid = D.objid
	     AND D.refobjid = T.oid
	     AND D.refobjid = C.attrelid
	     AND D.refobjsubid = C.attnum
	     AND T.relname = PGT.tablename
	 ORDER BY S.relname;`)
		if err != nil {
			fmt.Printf("Failed to generate sequence update: %v\n", err)
			return err
		}
		for _, r := range results {
			for _, value := range r {
				_, err = e.Exec(value)
				if err != nil {
					fmt.Printf("Failed to update sequence: %s Error: %v\n", value, err)
					return err
				}
			}
		}
	}
	_ = hash.Register("dummy", hash.NewDummyHasher)
	setting.PasswordHashAlgo, _ = hash.SetDefaultPasswordHashAlgorithm("dummy")

	return err
}

func isFieldNil(field reflect.Value) bool {
	if !field.IsValid() {
		return true
	}

	switch field.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return field.IsNil()
	default:
		return false
	}
}

func isFieldPrivate(field reflect.StructField) bool {
	return field.PkgPath != ""
}

func defaultFixtureDumperVerbs(tableName string, actualValue reflect.Value, typeOfactualValue reflect.Type, fd io.Writer, mapper names.Mapper, fieldDumper db.FixtureFieldDumper) error {
	for i := 0; i < actualValue.NumField(); i++ {
		field := actualValue.Field(i)

		fieldNameReal := actualValue.Type().Field(i).Name
		fieldName := mapper.Obj2Table(fieldNameReal)
		fieldType := typeOfactualValue.Field(i)
		if isFieldPrivate(fieldType) {
			continue
		}
		xormTags, err := splitXormTag(fieldType.Tag.Get("xorm"))
		if err != nil {
			return err
		}

		if xormTags.HasTag("-") {
			continue
		}

		if fieldDumper != nil {
			value, err := fieldDumper.FixtureFieldDumper(fieldNameReal)
			if err == nil {
				_, err = fd.Write([]byte(fmt.Sprintf("  %s: ", xormTags.GetFieldName(fieldName))))
				if err != nil {
					return err
				}

				_, err = fd.Write(value)
				if err != nil {
					return err
				}

				_, err = fd.Write([]byte("\n"))
				if err != nil {
					return err
				}

				continue
			}

			if err == db.ErrFixtureFieldDumperSkip {
				continue
			}

			if err != db.ErrFixtureFieldDumperContinue {
				return err
			}
		}

		fieldValue := field.Interface()
		isText := xormTags.HasTag("TEXT")
		isJSON := xormTags.HasTag("JSON")
		conversion, hasconversion := fieldValue.(convert.Conversion)
		if (!hasconversion) && isFieldNil(field) {
			continue
		}

		if isText {
			if _, ok := fieldValue.([]string); ok {
				isJSON = true
			}
		}

		if !isText {
			if strValue, ok := fieldValue.(string); ok {
				if len(strValue) == 0 {
					continue
				}
			}
		}

		if xormTags.HasTag("EXTENDS") {
			var actualValue2 reflect.Value

			if field.Kind() == reflect.Ptr {
				actualValue2 = field.Elem()
			} else {
				actualValue2 = field
			}

			typeOfactualValue2 := actualValue2.Type()
			err = defaultFixtureDumperVerbs(tableName, actualValue2, typeOfactualValue2, fd, mapper, fieldDumper)
			if err != nil {
				return err
			}

			continue
		}

		int64Type := reflect.TypeOf(int64(0))
		isInt64 := field.Type().ConvertibleTo(int64Type)
		if fieldType.Type.Kind() == reflect.Struct && !isInt64 && !isText && !isJSON {
			return fmt.Errorf("%s: '%s' is a struct which can't be convert to a table field", tableName, xormTags.GetFieldName(fieldName))
		}

		_, err = fd.Write([]byte(fmt.Sprintf("  %s: ", xormTags.GetFieldName(fieldName))))
		if err != nil {
			return err
		}

		if isJSON {
			result, err := json.Marshal(fieldValue)
			if err != nil {
				return err
			}
			result2 := []byte{'\''}
			result2 = append(result2, result...)
			result2 = append(result2, []byte{'\'', '\n'}...)

			_, err = fd.Write(result2)
			if err != nil {
				return err
			}

			continue
		}

		var strValue []byte
		if hasconversion {
			strValue, err = conversion.ToDB()
			if err != nil {
				return err
			}
		} else if isInt64 {
			intValue := field.Convert(int64Type).Int()
			strValue = []byte(fmt.Sprintf("%d", intValue))
		} else if bytes, ok := fieldValue.([]byte); ok && !isJSON {
			isText = false
			strValue = []byte{'0', 'x'}
			vStrBuilder := strings.Builder{}

			for _, b := range bytes {
				fmt.Fprintf(&vStrBuilder, "%02x", b)
			}

			strValue = append(strValue, []byte(vStrBuilder.String())...)

		} else {
			strValue = []byte(fmt.Sprintf("%v", fieldValue))
		}

		isAllNumber := true
		strValue2 := make([]byte, 0, len(strValue))
		for _, v := range strValue {
			if v == '\'' {
				isText = true
				strValue2 = append(strValue2, v)
			}

			if v == '#' {
				isText = true
			}

			if v < '0' || v > '9' {
				isAllNumber = false
			}

			strValue2 = append(strValue2, v)
		}

		if isAllNumber && !isInt64 {
			isText = true
		}

		if isText {
			strValue = []byte{'\''}
			strValue = append(strValue, strValue2...)
			strValue = append(strValue, '\'')
		} else {
			strValue = strValue2
		}
		strValue = append(strValue, '\n')

		_, err = fd.Write(strValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func DefaultFixtureDumper(data any, fd io.Writer) error {
	if data == nil {
		return nil
	}

	reflectedValue := reflect.ValueOf(data)

	if reflectedValue.Kind() != reflect.Ptr {
		return errors.New("expected a pointer")
	}

	fieldDumper, _ := data.(db.FixtureFieldDumper)

	actualValue := reflectedValue.Elem()
	typeOfactualValue := actualValue.Type()
	mapper := names.GonicMapper{}

	_, err := fd.Write([]byte("-\n"))
	if err != nil {
		return err
	}

	err = defaultFixtureDumperVerbs(typeOfactualValue.Name(), actualValue, typeOfactualValue, fd, mapper, fieldDumper)
	if err != nil {
		return err
	}

	_, err = fd.Write([]byte("\n"))
	if err != nil {
		return err
	}

	return nil
}

type xormTag struct {
	name   string
	params []string
}

type xormTagList []xormTag

func (l xormTagList) HasTag(name string) bool {
	for _, tag := range l {
		name2 := strings.ToUpper(tag.name)
		if name2 == name {
			return true
		}
	}

	return false
}

func (l xormTagList) GetFieldName(defaultName string) string {
	reservedNames := []string{
		"TRUE",
		"FALSE",
		"BIT",
		"TINYINT",
		"SMALLINT",
		"MEDIUMINT",
		"INT",
		"INTEGER",
		"BIGINT",
		"CHAR",
		"VARCHAR",
		"TINYTEXT",
		"TEXT",
		"MEDIUMTEXT",
		"LONGTEXT",
		"BINARY",
		"VARBINARY",
		"DATE",
		"DATETIME",
		"TIME",
		"TIMESTAMP",
		"TIMESTAMPZ",
		"REAL",
		"FLOAT",
		"DOUBLE",
		"DECIMAL",
		"NUMERIC",
		"TINYBLOB",
		"BLOB",
		"MEDIUMBLOB	",
		"LONGBLOB",
		"BYTEA",
		"BOOL",
		"SERIAL",
		"BIGSERIAL",
		"-",
		"<-",
		"->",
		"PK",
		"NULL",
		"NOT",
		"AUTOINCR",
		"DEFAULT",
		"CREATED",
		"UPDATED",
		"DELETED",
		"VERSION",
		"UTC",
		"LOCAL",
		"NOTNULL",
		"INDEX",
		"UNIQUE",
		"CACHE",
		"NOCACHE",
		"COMMENT",
		"EXTENDS",
		"UNSIGNED",
		"COLLATE",
		"JSON",
	}

	preTag := ""
	for _, tag := range l {
		if len(tag.params) > 0 {
			continue
		}

		if preTag == "DEFAULT" {
			preTag = ""
			continue
		}
		preTag = strings.ToUpper(tag.name)

		if util.SliceContainsString(reservedNames, strings.ToUpper(tag.name)) {
			continue
		}

		return tag.name
	}

	return defaultName
}

func splitXormTag(tagStr string) (xormTagList, error) {
	tagStr = strings.TrimSpace(tagStr)
	var (
		inQuote    bool
		inBigQuote bool
		lastIdx    int
		curTag     xormTag
		paramStart int
		tags       []xormTag
	)
	for i, t := range tagStr {
		switch t {
		case '\'':
			inQuote = !inQuote
		case ' ':
			if !inQuote && !inBigQuote {
				if lastIdx < i {
					if curTag.name == "" {
						curTag.name = tagStr[lastIdx:i]
					}
					tags = append(tags, curTag)
					lastIdx = i + 1
					curTag = xormTag{}
				} else if lastIdx == i {
					lastIdx = i + 1
				}
			} else if inBigQuote && !inQuote {
				paramStart = i + 1
			}
		case ',':
			if !inQuote && !inBigQuote {
				return nil, fmt.Errorf("comma[%d] of %s should be in quote or big quote", i, tagStr)
			}
			if !inQuote && inBigQuote {
				curTag.params = append(curTag.params, strings.TrimSpace(tagStr[paramStart:i]))
				paramStart = i + 1
			}
		case '(':
			inBigQuote = true
			if !inQuote {
				curTag.name = tagStr[lastIdx:i]
				paramStart = i + 1
			}
		case ')':
			inBigQuote = false
			if !inQuote {
				curTag.params = append(curTag.params, tagStr[paramStart:i])
			}
		}
	}
	if lastIdx < len(tagStr) {
		if curTag.name == "" {
			curTag.name = tagStr[lastIdx:]
		}
		tags = append(tags, curTag)
	}
	return tags, nil
}
