package transfer

import (
	"errors"
)

var (
	// ErrConflict is the conflict error.
	ErrConflict = errors.New("conflict")
	// ErrParseError is the parse error.
	ErrParseError = errors.New("parse error")
	// ErrMissingData is the missing data error.
	ErrMissingData = errors.New("missing data")
	// ErrExtraData is the extra data error.
	ErrExtraData = errors.New("extra data")
	// ErrCorruptData is the corrupt data error.
	ErrCorruptData = errors.New("corrupt data")
	// ErrNotAllowed is the not allowed error.
	ErrNotAllowed = errors.New("not allowed")
	// ErrInvalidPacket is the invalid packet error.
	ErrInvalidPacket = errors.New("invalid packet")
	// ErrNotFound is the not found error.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized is the unauthorized error.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrUnauthorized is the forbidden error.
	ErrForbidden = errors.New("forbidden")
)
