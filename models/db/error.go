// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/util"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	mssql "github.com/microsoft/go-mssqldb"
)

// ErrCancelled represents an error due to context cancellation
type ErrCancelled struct {
	Message string
}

// IsErrCancelled checks if an error is a ErrCancelled.
func IsErrCancelled(err error) bool {
	_, ok := err.(ErrCancelled)
	return ok
}

func (err ErrCancelled) Error() string {
	return "Cancelled: " + err.Message
}

// ErrCancelledf returns an ErrCancelled for the provided format and args
func ErrCancelledf(format string, args ...any) error {
	return ErrCancelled{
		fmt.Sprintf(format, args...),
	}
}

// ErrSSHDisabled represents an "SSH disabled" error.
type ErrSSHDisabled struct{}

// IsErrSSHDisabled checks if an error is a ErrSSHDisabled.
func IsErrSSHDisabled(err error) bool {
	_, ok := err.(ErrSSHDisabled)
	return ok
}

func (err ErrSSHDisabled) Error() string {
	return "SSH is disabled"
}

// ErrNotExist represents a non-exist error.
type ErrNotExist struct {
	Resource string
	ID       int64
}

// IsErrNotExist checks if an error is an ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	name := "record"
	if err.Resource != "" {
		name = err.Resource
	}

	if err.ID != 0 {
		return fmt.Sprintf("%s does not exist [id: %d]", name, err.ID)
	}
	return name + " does not exist"
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrNotExist) Unwrap() error {
	return util.ErrNotExist
}

// IsErrDuplicateKey checks if an error is a database unique constraint violation.
// This function properly detects unique constraint violations across all supported
// database systems (PostgreSQL, MySQL, SQLite, MSSQL) by checking database-specific
// error codes rather than relying on brittle string matching.
func IsErrDuplicateKey(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL: Check for error code 23505 (unique_violation)
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}

	// MySQL: Check for error number 1062 (ER_DUP_ENTRY)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}

	// SQLite: Check for SQLITE_CONSTRAINT error (handled in error_sqlite.go with build tags)
	if isErrDuplicateKeySQLite(err) {
		return true
	}

	// MSSQL: Check for error number 2627 (unique constraint violation) or 2601 (duplicate key)
	var mssqlErr mssql.Error
	if errors.As(err, &mssqlErr) {
		return mssqlErr.Number == 2627 || mssqlErr.Number == 2601
	}

	return false
}
