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

package terror

import (
	"fmt"
	"runtime"
	"strconv"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/mysql"
)

// Common base error instances.
var (
	CommitNotInTransaction   = ClassExecutor.New(CodeCommitNotInTransaction, "commit not in transaction")
	RollbackNotInTransaction = ClassExecutor.New(CodeRollbackNotInTransaction, "rollback not in transaction")
	ExecResultIsEmpty        = ClassExecutor.New(CodeExecResultIsEmpty, "exec result is empty")

	MissConnectionID = ClassExpression.New(CodeMissConnectionID, "miss connection id information")
)

// ErrCode represents a specific error type in a error class.
// Same error code can be used in different error classes.
type ErrCode int

// Executor error codes.
const (
	CodeCommitNotInTransaction   ErrCode = 1
	CodeRollbackNotInTransaction         = 2
	CodeExecResultIsEmpty                = 3
)

// Expression error codes.
const (
	CodeMissConnectionID ErrCode = iota + 1
)

// ErrClass represents a class of errors.
type ErrClass int

// Error classes.
const (
	ClassParser ErrClass = iota + 1
	ClassSchema
	ClassOptimizer
	ClassOptimizerPlan
	ClassExecutor
	ClassEvaluator
	ClassKV
	ClassServer
	ClassVariable
	ClassExpression
	// Add more as needed.
)

// String implements fmt.Stringer interface.
func (ec ErrClass) String() string {
	switch ec {
	case ClassParser:
		return "parser"
	case ClassSchema:
		return "schema"
	case ClassOptimizer:
		return "optimizer"
	case ClassExecutor:
		return "executor"
	case ClassKV:
		return "kv"
	case ClassServer:
		return "server"
	case ClassVariable:
		return "variable"
	case ClassExpression:
		return "expression"
	}
	return strconv.Itoa(int(ec))
}

// EqualClass returns true if err is *Error with the same class.
func (ec ErrClass) EqualClass(err error) bool {
	e := errors.Cause(err)
	if e == nil {
		return false
	}
	if te, ok := e.(*Error); ok {
		return te.class == ec
	}
	return false
}

// NotEqualClass returns true if err is not *Error with the same class.
func (ec ErrClass) NotEqualClass(err error) bool {
	return !ec.EqualClass(err)
}

// New creates an *Error with an error code and an error message.
// Usually used to create base *Error.
func (ec ErrClass) New(code ErrCode, message string) *Error {
	return &Error{
		class:   ec,
		code:    code,
		message: message,
	}
}

// Error implements error interface and adds integer Class and Code, so
// errors with different message can be compared.
type Error struct {
	class   ErrClass
	code    ErrCode
	message string
	file    string
	line    int
}

// Class returns ErrClass
func (e *Error) Class() ErrClass {
	return e.class
}

// Code returns ErrCode
func (e *Error) Code() ErrCode {
	return e.code
}

// Location returns the location where the error is created,
// implements juju/errors locationer interface.
func (e *Error) Location() (file string, line int) {
	return e.file, e.line
}

// Error implements error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("[%s:%d]%s", e.class, e.code, e.message)
}

// Gen generates a new *Error with the same class and code, and a new formatted message.
func (e *Error) Gen(format string, args ...interface{}) *Error {
	err := *e
	err.message = fmt.Sprintf(format, args...)
	_, err.file, err.line, _ = runtime.Caller(1)
	return &err
}

// Equal checks if err is equal to e.
func (e *Error) Equal(err error) bool {
	originErr := errors.Cause(err)
	if originErr == nil {
		return false
	}
	inErr, ok := originErr.(*Error)
	return ok && e.class == inErr.class && e.code == inErr.code
}

// NotEqual checks if err is not equal to e.
func (e *Error) NotEqual(err error) bool {
	return !e.Equal(err)
}

// ToSQLError convert Error to mysql.SQLError.
func (e *Error) ToSQLError() *mysql.SQLError {
	code := e.getMySQLErrorCode()
	return mysql.NewErrf(code, e.message)
}

var defaultMySQLErrorCode uint16

func (e *Error) getMySQLErrorCode() uint16 {
	codeMap, ok := ErrClassToMySQLCodes[e.class]
	if !ok {
		log.Warnf("Unknown error class: %v", e.class)
		return defaultMySQLErrorCode
	}
	code, ok := codeMap[e.code]
	if !ok {
		log.Warnf("Unknown error class: %v code: %v", e.class, e.code)
		return defaultMySQLErrorCode
	}
	return code
}

var (
	// ErrCode to mysql error code map.
	parserMySQLErrCodes     = map[ErrCode]uint16{}
	executorMySQLErrCodes   = map[ErrCode]uint16{}
	serverMySQLErrCodes     = map[ErrCode]uint16{}
	expressionMySQLErrCodes = map[ErrCode]uint16{}

	// ErrClassToMySQLCodes is the map of ErrClass to code-map.
	ErrClassToMySQLCodes map[ErrClass](map[ErrCode]uint16)
)

func init() {
	ErrClassToMySQLCodes = make(map[ErrClass](map[ErrCode]uint16))
	ErrClassToMySQLCodes[ClassParser] = parserMySQLErrCodes
	ErrClassToMySQLCodes[ClassExecutor] = executorMySQLErrCodes
	ErrClassToMySQLCodes[ClassServer] = serverMySQLErrCodes
	ErrClassToMySQLCodes[ClassExpression] = expressionMySQLErrCodes
	defaultMySQLErrorCode = mysql.ErrUnknown
}

// ErrorEqual returns a boolean indicating whether err1 is equal to err2.
func ErrorEqual(err1, err2 error) bool {
	e1 := errors.Cause(err1)
	e2 := errors.Cause(err2)

	if e1 == e2 {
		return true
	}

	if e1 == nil || e2 == nil {
		return e1 == e2
	}

	te1, ok1 := e1.(*Error)
	te2, ok2 := e2.(*Error)
	if ok1 && ok2 {
		return te1.class == te2.class && te1.code == te2.code
	}

	return e1.Error() == e2.Error()
}

// ErrorNotEqual returns a boolean indicating whether err1 isn't equal to err2.
func ErrorNotEqual(err1, err2 error) bool {
	return !ErrorEqual(err1, err2)
}
