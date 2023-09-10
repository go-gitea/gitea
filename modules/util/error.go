// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
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

// NewSilentWrapErrorf returns an error that formats as the given text but unwraps as the provided error
func NewSilentWrapErrorf(unwrap error, message string, args ...any) error {
	if len(args) == 0 {
		return SilentWrap{Message: message, Err: unwrap}
	}
	return SilentWrap{Message: fmt.Sprintf(message, args...), Err: unwrap}
}

// NewInvalidArgumentErrorf returns an error that formats as the given text but unwraps as an ErrInvalidArgument
func NewInvalidArgumentErrorf(message string, args ...any) error {
	return NewSilentWrapErrorf(ErrInvalidArgument, message, args...)
}

// NewPermissionDeniedErrorf returns an error that formats as the given text but unwraps as an ErrPermissionDenied
func NewPermissionDeniedErrorf(message string, args ...any) error {
	return NewSilentWrapErrorf(ErrPermissionDenied, message, args...)
}

// NewAlreadyExistErrorf returns an error that formats as the given text but unwraps as an ErrAlreadyExist
func NewAlreadyExistErrorf(message string, args ...any) error {
	return NewSilentWrapErrorf(ErrAlreadyExist, message, args...)
}

// NewNotExistErrorf returns an error that formats as the given text but unwraps as an ErrNotExist
func NewNotExistErrorf(message string, args ...any) error {
	return NewSilentWrapErrorf(ErrNotExist, message, args...)
}
