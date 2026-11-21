// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"fmt"
	"html"
	"html/template"
)

// This file defines common errors forming the base of our error system.
// These errors can be used to classify errors and to provide a common,
// safe (no server-side sensitive information), and translatable error message for end users.

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

// ErrorTranslateHTML translates the error message to HTML using the provided locale translator.
// If the error is not translatable, it returns false.
func ErrorTranslateHTML(err error, locale ErrorLocaleTranslator) (template.HTML, bool) {
	if errAs := errorAsTranslatable(err); errAs != nil {
		return locale.Tr(errAs.trKey, errAs.trArgs...), true
	}
	return "", false
}

// ErrorUnwrapForHTML unwraps the user error and returns the error message in HTML format and the HTTP status code.
// An error is considered a user error if it is "errorForUser" or "errorTranslatable"
// * errorTranslatable: The error message is translated to HTML using the provided locale translator.
// * errorForUser: The error message is returned as is (escaped to HTML) and the HTTP status code is returned.
// If the error is not a user error, it returns an empty message and code 0.
func ErrorUnwrapForHTML(err error, locale ErrorLocaleTranslator) (msg template.HTML, code int) {
	if e, ok := errors.AsType[errorForUser](err); ok {
		code = e.code
	}
	if htmlMsg, ok := ErrorTranslateHTML(err, locale); ok {
		msg = htmlMsg
		code = IfZero(code, 400)
	}
	if code != 0 && msg == "" {
		errMsg := err.Error()
		switch errMsg {
		case ErrPermissionDenied.msg:
			msg = locale.Tr("error.permission_denied")
		case ErrNotExist.msg:
			msg = locale.Tr("error.not_found")
		default:
			msg = template.HTML(html.EscapeString(errMsg))
		}
	}
	return msg, code
}

// ErrorUnwrapForUser unwraps the user error and returns the error message in string format and the HTTP status code.
// It is mainly used for API error responses, and ideally only English locale should be used.
//
// It also accepts a locale translator to translate the error message if it is translatable:
// * for example: there are different error messages for "ErrBlockedUser": blocked by owner, or blocked by assignee
// * only the function returned the error message can know the exact reason
func ErrorUnwrapForUser(err error, locale ...ErrorLocaleTranslator) (msg string, code int) {
	e, ok := errors.AsType[errorForUser](err)
	if ok {
		msg, code = err.Error(), e.code
	}
	if len(locale) > 0 {
		if errTr := errorAsTranslatable(err); errTr != nil {
			msg = locale[0].TrString(errTr.trKey, errTr.trArgs...)
			code = IfZero(code, 400)
		}
	}
	return msg, code
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

type errorTranslatable struct {
	err    error
	trKey  string
	trArgs []any
}

type ErrorLocaleTranslator interface {
	Tr(key string, args ...any) template.HTML
	TrString(key string, args ...any) string
}

func (w *errorTranslatable) Error() string { return w.err.Error() }
func (w *errorTranslatable) Unwrap() error { return w.err }

func ErrorWrapTranslatable(err error, trKey string, trArgs ...any) error {
	return &errorTranslatable{err: err, trKey: trKey, trArgs: trArgs}
}

func errorAsTranslatable(err error) *errorTranslatable {
	e, _ := errors.AsType[*errorTranslatable](err)
	return e
}
