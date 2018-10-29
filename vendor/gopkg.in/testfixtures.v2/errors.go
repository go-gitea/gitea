package testfixtures

import (
	"errors"
	"fmt"
)

var (
	// ErrWrongCastNotAMap is returned when a map is not a map[interface{}]interface{}
	ErrWrongCastNotAMap = errors.New("Could not cast record: not a map[interface{}]interface{}")

	// ErrFileIsNotSliceOrMap is returned the the fixture file is not a slice or map.
	ErrFileIsNotSliceOrMap = errors.New("The fixture file is not a slice or map")

	// ErrKeyIsNotString is returned when a record is not of type string
	ErrKeyIsNotString = errors.New("Record map key is not string")

	// ErrNotTestDatabase is returned when the database name doesn't contains "test"
	ErrNotTestDatabase = errors.New(`Loading aborted because the database name does not contains "test"`)
)

// InsertError will be returned if any error happens on database while
// inserting the record
type InsertError struct {
	Err    error
	File   string
	Index  int
	SQL    string
	Params []interface{}
}

func (e *InsertError) Error() string {
	return fmt.Sprintf(
		"testfixtures: error inserting record: %v, on file: %s, index: %d, sql: %s, params: %v",
		e.Err,
		e.File,
		e.Index,
		e.SQL,
		e.Params,
	)
}
