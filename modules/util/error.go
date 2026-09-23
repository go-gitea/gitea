// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
	"html/template"
)

// This file defines common errors forming the base of our error system.
// These errors can be used to classify errors and to provide a common,
// safe (no server-side sensitive information), and translatable error message for end users.

// ErrorTranslatable wraps an error with translation information
type ErrorTranslatable interface {
	error
	Unwrap() error
	Translate(ErrorLocaleTranslator) template.HTML
}

var (
	ErrInvalidArgument  = errorForUser{400, "invalid argument"}
	ErrPermissionDenied = errorForUser{403, "permission denied"}
	ErrNotExist         = errorForUser{404, "resource does not exist"}
	ErrAlreadyExist     = errorForUser{409, "resource already exists"} // 409 Conflict
	ErrContentTooLarge  = errorForUser{413, "content exceeds limit"}   // 413 Request Entity Too Large

	// ErrUnprocessableContent means request content is correct, but the server is unable to process the contained instructions
	ErrUnprocessableContent = errorForUser{422, "unprocessable content"} // 422 Unprocessable Entity
)

type errorForUser struct {
	code int // implies HTTP status code
	msg  string
}

func (w errorForUser) Error() string {
	return w.msg
}

func ErrorUnwrapForUser(err error) (string, int) {
	if e, ok := errors.AsType[errorForUser](err); ok {
		return err.Error(), e.code
	}
	return "", 0
}

// errorWrapper provides a simple wrapper for a wrapped error where the wrapped error message plays no part in the error message
type errorWrapper struct {
	msg string
	err error
}

func (w errorWrapper) Error() string {
	return w.msg
}

func (w errorWrapper) Unwrap() error {
	return w.err
}

// ErrorWrap returns an error that formats as the given text but unwraps as the provided error
// The message should be safe (no sensitive information) to be shown to end users
func ErrorWrap(unwrap error, message string, args ...any) error {
	if len(args) == 0 {
		return errorWrapper{msg: message, err: unwrap}
	}
	return errorWrapper{msg: fmt.Sprintf(message, args...), err: unwrap}
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
	if e, ok := errors.AsType[*errorTranslatableWrapper](err); ok {
		return e
	}
	return nil
}
