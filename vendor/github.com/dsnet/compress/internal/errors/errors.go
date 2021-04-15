// Copyright 2016, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package errors implements functions to manipulate compression errors.
//
// In idiomatic Go, it is an anti-pattern to use panics as a form of error
// reporting in the API. Instead, the expected way to transmit errors is by
// returning an error value. Unfortunately, the checking of "err != nil" in
// tight loops commonly found in compression causes non-negligible performance
// degradation. While this may not be idiomatic, the internal packages of this
// repository rely on panics as a normal means to convey errors. In order to
// ensure that these panics do not leak across the public API, the public
// packages must recover from these panics and present an error value.
//
// The Panic and Recover functions in this package provide a safe way to
// recover from errors only generated from within this repository.
//
// Example usage:
//	func Foo() (err error) {
//		defer errors.Recover(&err)
//
//		if rand.Intn(2) == 0 {
//			// Unexpected panics will not be caught by Recover.
//			io.Closer(nil).Close()
//		} else {
//			// Errors thrown by Panic will be caught by Recover.
//			errors.Panic(errors.New("whoopsie"))
//		}
//	}
//
package errors

import "strings"

const (
	// Unknown indicates that there is no classification for this error.
	Unknown = iota

	// Internal indicates that this error is due to an internal bug.
	// Users should file a issue report if this type of error is encountered.
	Internal

	// Invalid indicates that this error is due to the user misusing the API
	// and is indicative of a bug on the user's part.
	Invalid

	// Deprecated indicates the use of a deprecated and unsupported feature.
	Deprecated

	// Corrupted indicates that the input stream is corrupted.
	Corrupted

	// Closed indicates that the handlers are closed.
	Closed
)

var codeMap = map[int]string{
	Unknown:    "unknown error",
	Internal:   "internal error",
	Invalid:    "invalid argument",
	Deprecated: "deprecated format",
	Corrupted:  "corrupted input",
	Closed:     "closed handler",
}

type Error struct {
	Code int    // The error type
	Pkg  string // Name of the package where the error originated
	Msg  string // Descriptive message about the error (optional)
}

func (e Error) Error() string {
	var ss []string
	for _, s := range []string{e.Pkg, codeMap[e.Code], e.Msg} {
		if s != "" {
			ss = append(ss, s)
		}
	}
	return strings.Join(ss, ": ")
}

func (e Error) CompressError()     {}
func (e Error) IsInternal() bool   { return e.Code == Internal }
func (e Error) IsInvalid() bool    { return e.Code == Invalid }
func (e Error) IsDeprecated() bool { return e.Code == Deprecated }
func (e Error) IsCorrupted() bool  { return e.Code == Corrupted }
func (e Error) IsClosed() bool     { return e.Code == Closed }

func IsInternal(err error) bool   { return isCode(err, Internal) }
func IsInvalid(err error) bool    { return isCode(err, Invalid) }
func IsDeprecated(err error) bool { return isCode(err, Deprecated) }
func IsCorrupted(err error) bool  { return isCode(err, Corrupted) }
func IsClosed(err error) bool     { return isCode(err, Closed) }

func isCode(err error, code int) bool {
	if cerr, ok := err.(Error); ok && cerr.Code == code {
		return true
	}
	return false
}

// errWrap is used by Panic and Recover to ensure that only errors raised by
// Panic are recovered by Recover.
type errWrap struct{ e *error }

func Recover(err *error) {
	switch ex := recover().(type) {
	case nil:
		// Do nothing.
	case errWrap:
		*err = *ex.e
	default:
		panic(ex)
	}
}

func Panic(err error) {
	panic(errWrap{&err})
}
