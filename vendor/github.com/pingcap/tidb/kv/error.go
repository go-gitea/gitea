// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import (
	"errors"
	"strings"

	"github.com/pingcap/go-themis"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/terror"
)

// KV error codes.
const (
	CodeIncompatibleDBFormat terror.ErrCode = 1
	CodeNoDataForHandle      terror.ErrCode = 2
	CodeKeyExists            terror.ErrCode = 3
)

var (
	// ErrClosed is used when close an already closed txn.
	ErrClosed = errors.New("Error: Transaction already closed")
	// ErrNotExist is used when try to get an entry with an unexist key from KV store.
	ErrNotExist = errors.New("Error: key not exist")
	// ErrConditionNotMatch is used when condition is not met.
	ErrConditionNotMatch = errors.New("Error: Condition not match")
	// ErrLockConflict is used when try to lock an already locked key.
	ErrLockConflict = errors.New("Error: Lock conflict")
	// ErrLazyConditionPairsNotMatch is used when value in store differs from expect pairs.
	ErrLazyConditionPairsNotMatch = errors.New("Error: Lazy condition pairs not match")
	// ErrRetryable is used when KV store occurs RPC error or some other
	// errors which SQL layer can safely retry.
	ErrRetryable = errors.New("Error: KV error safe to retry")
	// ErrCannotSetNilValue is the error when sets an empty value.
	ErrCannotSetNilValue = errors.New("can not set nil value")
	// ErrInvalidTxn is the error when commits or rollbacks in an invalid transaction.
	ErrInvalidTxn = errors.New("invalid transaction")

	// ErrNotCommitted is the error returned by CommitVersion when this
	// transaction is not committed.
	ErrNotCommitted = errors.New("this transaction has not committed")

	// ErrKeyExists returns when key is already exist.
	ErrKeyExists = terror.ClassKV.New(CodeKeyExists, "key already exist")
)

func init() {
	kvMySQLErrCodes := map[terror.ErrCode]uint16{
		CodeKeyExists: mysql.ErrDupEntry,
	}
	terror.ErrClassToMySQLCodes[terror.ClassKV] = kvMySQLErrCodes
}

// IsRetryableError checks if the err is a fatal error and the under going operation is worth to retry.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if terror.ErrorEqual(err, ErrRetryable) ||
		terror.ErrorEqual(err, ErrLockConflict) ||
		terror.ErrorEqual(err, ErrConditionNotMatch) ||
		terror.ErrorEqual(err, themis.ErrRetryable) ||
		// HBase exception message will tell you if you should retry or not
		strings.Contains(err.Error(), "try again later") {
		return true
	}

	return false
}

// IsErrNotFound checks if err is a kind of NotFound error.
func IsErrNotFound(err error) bool {
	if terror.ErrorEqual(err, ErrNotExist) {
		return true
	}

	return false
}
