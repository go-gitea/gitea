// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"io/fs"
)

// Common Errors forming the base of our error system
//
// Many Errors returned by Gitea can be tested against these errors
// using errors.Is.
var (
	ErrInvalid    = fs.ErrInvalid    // "invalid argument"
	ErrPermission = fs.ErrPermission // "permission denied"
	ErrExist      = fs.ErrExist      // "file already exists"
	ErrNotExist   = fs.ErrNotExist   // "file does not exist"
)

// SilentWrap provides a simple wrapper for a wrapped error where the wrapped error message plays no part in the error message
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
