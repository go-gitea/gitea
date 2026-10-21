// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"
	"fmt"

	"gitea.dev/modules/util"
)

// ErrCancelled represents an error due to context cancellation
type ErrCancelled struct {
	Message string
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

// sqlStateCarrier is implemented by driver errors that report a SQLSTATE.
type sqlStateCarrier interface{ SQLState() string }

// IsConflictError reports whether the database rejected the statement because it conflicted with a
// concurrent transaction (SQLSTATE 40001). Firebird raises it when an update loses a race: rather
// than re-evaluating the condition and reporting fewer affected rows it reports the conflict as an
// error, so a caller that guards an update by its affected count has to handle it.
func IsConflictError(err error) bool {
	var carrier sqlStateCarrier
	return errors.As(err, &carrier) && carrier.SQLState() == "40001"
}
