// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"errors"
)

// Common Errors forming the base of our error system
//
// Many Errors returned by Gitea can be tested against these errors
// using errors.Is.
var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrPermissionDenied = errors.New("permission denied")
	ErrAlreadyExist     = errors.New("resource already exists")
	ErrNotExist         = errors.New("resource does not exist")
)

// SilentWrap provides a simple wrapper for a wrapped error where the wrapped error message plays no part in the error message
// Especially useful for "untyped" errors created with "errors.New(…)" that can be classified as 'invalid argument', 'permission denied', 'exists already', or 'does not exist'
type SilentWrap struct {
	Message string
	Err     error
}

// Error returns the message
func (w SilentWrap) Error() string {
	return w.Message
}

// Unwrap returns the underlying error
func (w SilentWrap) Unwrap() error {
	return w.Err
}
