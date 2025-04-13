// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
)

// Common Errors forming the base of our error system
//
// Many Errors returned by Gitea can be tested against these errors using "errors.Is".
var (
	ErrInvalidArgument  = errors.New("invalid argument")        // also implies HTTP 400
	ErrPermissionDenied = errors.New("permission denied")       // also implies HTTP 403
	ErrNotExist         = errors.New("resource does not exist") // also implies HTTP 404
	ErrAlreadyExist     = errors.New("resource already exists") // also implies HTTP 409

	// ErrUnprocessableContent implies HTTP 422, syntax of the request content was correct,
	// but server was unable to process the contained instructions
	ErrUnprocessableContent = errors.New("unprocessable content")
)

// errorWrapper provides a simple wrapper for a wrapped error where the wrapped error message plays no part in the error message
// Especially useful for "untyped" errors created with "errors.New(…)" that can be classified as 'invalid argument', 'permission denied', 'exists already', or 'does not exist'
type errorWrapper struct {
	Message string
	Err     error
}

// Error returns the message
func (w errorWrapper) Error() string {
	return w.Message
}

// Unwrap returns the underlying error
func (w errorWrapper) Unwrap() error {
	return w.Err
}

type LocaleWrapper struct {
	err    error
	TrKey  string
	TrArgs []any
}

// Error returns the message
func (w LocaleWrapper) Error() string {
	return w.err.Error()
}

// Unwrap returns the underlying error
func (w LocaleWrapper) Unwrap() error {
	return w.err
}

// ErrorWrap returns an error that formats as the given text but unwraps as the provided error
func ErrorWrap(unwrap error, message string, args ...any) error {
	if len(args) == 0 {
		return errorWrapper{Message: message, Err: unwrap}
	}
	return errorWrapper{Message: fmt.Sprintf(message, args...), Err: unwrap}
}

// NewInvalidArgumentErrorf returns an error that formats as the given text but unwraps as an ErrInvalidArgument
func NewInvalidArgumentErrorf(message string, args ...any) error {
	return ErrorWrap(ErrInvalidArgument, message, args...)
}

// NewPermissionDeniedErrorf returns an error that formats as the given text but unwraps as an ErrPermissionDenied
func NewPermissionDeniedErrorf(message string, args ...any) error {
	return ErrorWrap(ErrPermissionDenied, message, args...)
}

// NewAlreadyExistErrorf returns an error that formats as the given text but unwraps as an ErrAlreadyExist
func NewAlreadyExistErrorf(message string, args ...any) error {
	return ErrorWrap(ErrAlreadyExist, message, args...)
}

// NewNotExistErrorf returns an error that formats as the given text but unwraps as an ErrNotExist
func NewNotExistErrorf(message string, args ...any) error {
	return ErrorWrap(ErrNotExist, message, args...)
}

// ErrorWrapLocale wraps an err with a translation key and arguments
func ErrorWrapLocale(err error, trKey string, trArgs ...any) error {
	return LocaleWrapper{err: err, TrKey: trKey, TrArgs: trArgs}
}

func ErrorAsLocale(err error) *LocaleWrapper {
	var e LocaleWrapper
	if errors.As(err, &e) {
		return &e
	}
	return nil
}
