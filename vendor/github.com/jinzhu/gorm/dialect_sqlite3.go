package gorm

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type sqlite3 struct {
	commonDialect
}

func init() {
	RegisterDialect("sqlite3", &sqlite3{})
}

func (sqlite3) GetName() string {
	return "sqlite3"
}

// Get Data Type for Sqlite Dialect
func (s *sqlite3) DataTypeOf(field *StructField) string {
	var dataValue, sqlType, size, additionalType = ParseFieldStructForDialect(field, s)

	if sqlType == "" {
		switch dataValue.Kind() {
		case reflect.Bool:
			sqlType = "bool"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
			if s.fieldCanAutoIncrement(field) {
				field.TagSettingsSet("AUTO_INCREMENT", "AUTO_INCREMENT")
				sqlType = "integer primary key autoincrement"
			} else {
				sqlType = "integer"
			}
		case reflect.Int64, reflect.Uint64:
			if s.fieldCanAutoIncrement(field) {
				field.TagSettingsSet("AUTO_INCREMENT", "AUTO_INCREMENT")
				sqlType = "integer primary key autoincrement"
			} else {
				sqlType = "bigint"
			}
		case reflect.Float32, reflect.Float64:
			sqlType = "real"
		case reflect.String:
			if size > 0 && size < 65532 {
				sqlType = fmt.Sprintf("varchar(%d)", size)
			} else {
				sqlType = "text"
			}
		case reflect.Struct:
			if _, ok := dataValue.Interface().(time.Time); ok {
				sqlType = "datetime"
			}
		default:
			if IsByteArrayOrSlice(dataValue) {
				sqlType = "blob"
			}
		}
	}

	if sqlType == "" {
		panic(fmt.Sprintf("invalid sql type %s (%s) for sqlite3", dataValue.Type().Name(), dataValue.Kind().String()))
	}

	if strings.TrimSpace(additionalType) == "" {
		return sqlType
	}
	return fmt.Sprintf("%v %v", sqlType, additionalType)
}

func (s sqlite3) HasIndex(tableName string, indexName string) bool {
	var count int
	s.db.QueryRow(fmt.Sprintf("SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND sql LIKE '%%INDEX %v ON%%'", indexName), tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasTable(tableName string) bool {
	var count int
	s.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) HasColumn(tableName string, columnName string) bool {
	var count int
	s.db.QueryRow(fmt.Sprintf("SELECT count(*) FROM sqlite_master WHERE tbl_name = ? AND (sql LIKE '%%\"%v\" %%' OR sql LIKE '%%%v %%');\n", columnName, columnName), tableName).Scan(&count)
	return count > 0
}

func (s sqlite3) CurrentDatabase() (name string) {
	var (
		ifaces   = make([]interface{}, 3)
		pointers = make([]*string, 3)
		i        int
	)
	for i = 0; i < 3; i++ {
		ifaces[i] = &pointers[i]
	}
	if err := s.db.QueryRow("PRAGMA database_list").Scan(ifaces...); err != nil {
		return
	}
	if pointers[1] != nil {
		name = *pointers[1]
	}
	return
}
