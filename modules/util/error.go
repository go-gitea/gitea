// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
	"html/template"
)

// Common Errors forming the base of our error system
//
// Many Errors returned by Gitea can be tested against these errors using "errors.Is".
var (
	ErrInvalidArgument  = errors.New("invalid argument")        // also implies HTTP 400
	ErrPermissionDenied = errors.New("permission denied")       // also implies HTTP 403
	ErrNotExist         = errors.New("resource does not exist") // also implies HTTP 404
	ErrAlreadyExist     = errors.New("resource already exists") // also implies HTTP 409
	ErrContentTooLarge  = errors.New("content exceeds limit")   // also implies HTTP 413

	// ErrUnprocessableContent implies HTTP 422, the syntax of the request content is correct,
	// but the server is unable to process the contained instructions
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

// ErrorTranslatable wraps an error with translation information
type ErrorTranslatable interface {
	error
	Unwrap() error
	Translate(ErrorLocaleTranslator) template.HTML
}

type errorTranslatableWrapper struct {
	err    error
	trKey  string
	trArgs []any
}

type ErrorLocaleTranslator interface {
	Tr(key string, args ...any) template.HTML
}

func (w *errorTranslatableWrapper) Error() string { return w.err.Error() }

func (w *errorTranslatableWrapper) Unwrap() error { return w.err }

func (w *errorTranslatableWrapper) Translate(t ErrorLocaleTranslator) template.HTML {
	return t.Tr(w.trKey, w.trArgs...)
}

func ErrorWrapTranslatable(err error, trKey string, trArgs ...any) ErrorTranslatable {
	return &errorTranslatableWrapper{err: err, trKey: trKey, trArgs: trArgs}
}

func ErrorAsTranslatable(err error) ErrorTranslatable {
	var e *errorTranslatableWrapper
	if errors.As(err, &e) {
		return e
	}
	return nil
}
