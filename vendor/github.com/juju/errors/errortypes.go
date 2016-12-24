// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package errors

import (
	"fmt"
)

// wrap is a helper to construct an *wrapper.
func wrap(err error, format, suffix string, args ...interface{}) Err {
	newErr := Err{
		message:  fmt.Sprintf(format+suffix, args...),
		previous: err,
	}
	newErr.SetLocation(2)
	return newErr
}

// notFound represents an error when something has not been found.
type notFound struct {
	Err
}

// NotFoundf returns an error which satisfies IsNotFound().
func NotFoundf(format string, args ...interface{}) error {
	return &notFound{wrap(nil, format, " not found", args...)}
}

// NewNotFound returns an error which wraps err that satisfies
// IsNotFound().
func NewNotFound(err error, msg string) error {
	return &notFound{wrap(err, msg, "")}
}

// IsNotFound reports whether err was created with NotFoundf() or
// NewNotFound().
func IsNotFound(err error) bool {
	err = Cause(err)
	_, ok := err.(*notFound)
	return ok
}

// userNotFound represents an error when an inexistent user is looked up.
type userNotFound struct {
	Err
}

// UserNotFoundf returns an error which satisfies IsUserNotFound().
func UserNotFoundf(format string, args ...interface{}) error {
	return &userNotFound{wrap(nil, format, " user not found", args...)}
}

// NewUserNotFound returns an error which wraps err and satisfies
// IsUserNotFound().
func NewUserNotFound(err error, msg string) error {
	return &userNotFound{wrap(err, msg, "")}
}

// IsUserNotFound reports whether err was created with UserNotFoundf() or
// NewUserNotFound().
func IsUserNotFound(err error) bool {
	err = Cause(err)
	_, ok := err.(*userNotFound)
	return ok
}

// unauthorized represents an error when an operation is unauthorized.
type unauthorized struct {
	Err
}

// Unauthorizedf returns an error which satisfies IsUnauthorized().
func Unauthorizedf(format string, args ...interface{}) error {
	return &unauthorized{wrap(nil, format, "", args...)}
}

// NewUnauthorized returns an error which wraps err and satisfies
// IsUnauthorized().
func NewUnauthorized(err error, msg string) error {
	return &unauthorized{wrap(err, msg, "")}
}

// IsUnauthorized reports whether err was created with Unauthorizedf() or
// NewUnauthorized().
func IsUnauthorized(err error) bool {
	err = Cause(err)
	_, ok := err.(*unauthorized)
	return ok
}

// notImplemented represents an error when something is not
// implemented.
type notImplemented struct {
	Err
}

// NotImplementedf returns an error which satisfies IsNotImplemented().
func NotImplementedf(format string, args ...interface{}) error {
	return &notImplemented{wrap(nil, format, " not implemented", args...)}
}

// NewNotImplemented returns an error which wraps err and satisfies
// IsNotImplemented().
func NewNotImplemented(err error, msg string) error {
	return &notImplemented{wrap(err, msg, "")}
}

// IsNotImplemented reports whether err was created with
// NotImplementedf() or NewNotImplemented().
func IsNotImplemented(err error) bool {
	err = Cause(err)
	_, ok := err.(*notImplemented)
	return ok
}

// alreadyExists represents and error when something already exists.
type alreadyExists struct {
	Err
}

// AlreadyExistsf returns an error which satisfies IsAlreadyExists().
func AlreadyExistsf(format string, args ...interface{}) error {
	return &alreadyExists{wrap(nil, format, " already exists", args...)}
}

// NewAlreadyExists returns an error which wraps err and satisfies
// IsAlreadyExists().
func NewAlreadyExists(err error, msg string) error {
	return &alreadyExists{wrap(err, msg, "")}
}

// IsAlreadyExists reports whether the error was created with
// AlreadyExistsf() or NewAlreadyExists().
func IsAlreadyExists(err error) bool {
	err = Cause(err)
	_, ok := err.(*alreadyExists)
	return ok
}

// notSupported represents an error when something is not supported.
type notSupported struct {
	Err
}

// NotSupportedf returns an error which satisfies IsNotSupported().
func NotSupportedf(format string, args ...interface{}) error {
	return &notSupported{wrap(nil, format, " not supported", args...)}
}

// NewNotSupported returns an error which wraps err and satisfies
// IsNotSupported().
func NewNotSupported(err error, msg string) error {
	return &notSupported{wrap(err, msg, "")}
}

// IsNotSupported reports whether the error was created with
// NotSupportedf() or NewNotSupported().
func IsNotSupported(err error) bool {
	err = Cause(err)
	_, ok := err.(*notSupported)
	return ok
}

// notValid represents an error when something is not valid.
type notValid struct {
	Err
}

// NotValidf returns an error which satisfies IsNotValid().
func NotValidf(format string, args ...interface{}) error {
	return &notValid{wrap(nil, format, " not valid", args...)}
}

// NewNotValid returns an error which wraps err and satisfies IsNotValid().
func NewNotValid(err error, msg string) error {
	return &notValid{wrap(err, msg, "")}
}

// IsNotValid reports whether the error was created with NotValidf() or
// NewNotValid().
func IsNotValid(err error) bool {
	err = Cause(err)
	_, ok := err.(*notValid)
	return ok
}

// notProvisioned represents an error when something is not yet provisioned.
type notProvisioned struct {
	Err
}

// NotProvisionedf returns an error which satisfies IsNotProvisioned().
func NotProvisionedf(format string, args ...interface{}) error {
	return &notProvisioned{wrap(nil, format, " not provisioned", args...)}
}

// NewNotProvisioned returns an error which wraps err that satisfies
// IsNotProvisioned().
func NewNotProvisioned(err error, msg string) error {
	return &notProvisioned{wrap(err, msg, "")}
}

// IsNotProvisioned reports whether err was created with NotProvisionedf() or
// NewNotProvisioned().
func IsNotProvisioned(err error) bool {
	err = Cause(err)
	_, ok := err.(*notProvisioned)
	return ok
}

// notAssigned represents an error when something is not yet assigned to
// something else.
type notAssigned struct {
	Err
}

// NotAssignedf returns an error which satisfies IsNotAssigned().
func NotAssignedf(format string, args ...interface{}) error {
	return &notAssigned{wrap(nil, format, " not assigned", args...)}
}

// NewNotAssigned returns an error which wraps err that satisfies
// IsNotAssigned().
func NewNotAssigned(err error, msg string) error {
	return &notAssigned{wrap(err, msg, "")}
}

// IsNotAssigned reports whether err was created with NotAssignedf() or
// NewNotAssigned().
func IsNotAssigned(err error) bool {
	err = Cause(err)
	_, ok := err.(*notAssigned)
	return ok
}

// badRequest represents an error when a request has bad parameters.
type badRequest struct {
	Err
}

// BadRequestf returns an error which satisfies IsBadRequest().
func BadRequestf(format string, args ...interface{}) error {
	return &badRequest{wrap(nil, format, "", args...)}
}

// NewBadRequest returns an error which wraps err that satisfies
// IsBadRequest().
func NewBadRequest(err error, msg string) error {
	return &badRequest{wrap(err, msg, "")}
}

// IsBadRequest reports whether err was created with BadRequestf() or
// NewBadRequest().
func IsBadRequest(err error) bool {
	err = Cause(err)
	_, ok := err.(*badRequest)
	return ok
}

// methodNotAllowed represents an error when an HTTP request
// is made with an inappropriate method.
type methodNotAllowed struct {
	Err
}

// MethodNotAllowedf returns an error which satisfies IsMethodNotAllowed().
func MethodNotAllowedf(format string, args ...interface{}) error {
	return &methodNotAllowed{wrap(nil, format, "", args...)}
}

// NewMethodNotAllowed returns an error which wraps err that satisfies
// IsMethodNotAllowed().
func NewMethodNotAllowed(err error, msg string) error {
	return &methodNotAllowed{wrap(err, msg, "")}
}

// IsMethodNotAllowed reports whether err was created with MethodNotAllowedf() or
// NewMethodNotAllowed().
func IsMethodNotAllowed(err error) bool {
	err = Cause(err)
	_, ok := err.(*methodNotAllowed)
	return ok
}
