// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"fmt"
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
func ErrCancelledf(format string, args ...interface{}) error {
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
	ID int64
}

// IsErrNotExist checks if an error is an ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("record does not exist [id: %d]", err.ID)
}
