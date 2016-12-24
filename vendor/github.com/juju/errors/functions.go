// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package errors

import (
	"fmt"
	"strings"
)

// New is a drop in replacement for the standard libary errors module that records
// the location that the error is created.
//
// For example:
//    return errors.New("validation failed")
//
func New(message string) error {
	err := &Err{message: message}
	err.SetLocation(1)
	return err
}

// Errorf creates a new annotated error and records the location that the
// error is created.  This should be a drop in replacement for fmt.Errorf.
//
// For example:
//    return errors.Errorf("validation failed: %s", message)
//
func Errorf(format string, args ...interface{}) error {
	err := &Err{message: fmt.Sprintf(format, args...)}
	err.SetLocation(1)
	return err
}

// Trace adds the location of the Trace call to the stack.  The Cause of the
// resulting error is the same as the error parameter.  If the other error is
// nil, the result will be nil.
//
// For example:
//   if err := SomeFunc(); err != nil {
//       return errors.Trace(err)
//   }
//
func Trace(other error) error {
	if other == nil {
		return nil
	}
	err := &Err{previous: other, cause: Cause(other)}
	err.SetLocation(1)
	return err
}

// Annotate is used to add extra context to an existing error. The location of
// the Annotate call is recorded with the annotations. The file, line and
// function are also recorded.
//
// For example:
//   if err := SomeFunc(); err != nil {
//       return errors.Annotate(err, "failed to frombulate")
//   }
//
func Annotate(other error, message string) error {
	if other == nil {
		return nil
	}
	err := &Err{
		previous: other,
		cause:    Cause(other),
		message:  message,
	}
	err.SetLocation(1)
	return err
}

// Annotatef is used to add extra context to an existing error. The location of
// the Annotate call is recorded with the annotations. The file, line and
// function are also recorded.
//
// For example:
//   if err := SomeFunc(); err != nil {
//       return errors.Annotatef(err, "failed to frombulate the %s", arg)
//   }
//
func Annotatef(other error, format string, args ...interface{}) error {
	if other == nil {
		return nil
	}
	err := &Err{
		previous: other,
		cause:    Cause(other),
		message:  fmt.Sprintf(format, args...),
	}
	err.SetLocation(1)
	return err
}

// DeferredAnnotatef annotates the given error (when it is not nil) with the given
// format string and arguments (like fmt.Sprintf). If *err is nil, DeferredAnnotatef
// does nothing. This method is used in a defer statement in order to annotate any
// resulting error with the same message.
//
// For example:
//
//    defer DeferredAnnotatef(&err, "failed to frombulate the %s", arg)
//
func DeferredAnnotatef(err *error, format string, args ...interface{}) {
	if *err == nil {
		return
	}
	newErr := &Err{
		message:  fmt.Sprintf(format, args...),
		cause:    Cause(*err),
		previous: *err,
	}
	newErr.SetLocation(1)
	*err = newErr
}

// Wrap changes the Cause of the error. The location of the Wrap call is also
// stored in the error stack.
//
// For example:
//   if err := SomeFunc(); err != nil {
//       newErr := &packageError{"more context", private_value}
//       return errors.Wrap(err, newErr)
//   }
//
func Wrap(other, newDescriptive error) error {
	err := &Err{
		previous: other,
		cause:    newDescriptive,
	}
	err.SetLocation(1)
	return err
}

// Wrapf changes the Cause of the error, and adds an annotation. The location
// of the Wrap call is also stored in the error stack.
//
// For example:
//   if err := SomeFunc(); err != nil {
//       return errors.Wrapf(err, simpleErrorType, "invalid value %q", value)
//   }
//
func Wrapf(other, newDescriptive error, format string, args ...interface{}) error {
	err := &Err{
		message:  fmt.Sprintf(format, args...),
		previous: other,
		cause:    newDescriptive,
	}
	err.SetLocation(1)
	return err
}

// Mask masks the given error with the given format string and arguments (like
// fmt.Sprintf), returning a new error that maintains the error stack, but
// hides the underlying error type.  The error string still contains the full
// annotations. If you want to hide the annotations, call Wrap.
func Maskf(other error, format string, args ...interface{}) error {
	if other == nil {
		return nil
	}
	err := &Err{
		message:  fmt.Sprintf(format, args...),
		previous: other,
	}
	err.SetLocation(1)
	return err
}

// Mask hides the underlying error type, and records the location of the masking.
func Mask(other error) error {
	if other == nil {
		return nil
	}
	err := &Err{
		previous: other,
	}
	err.SetLocation(1)
	return err
}

// Cause returns the cause of the given error.  This will be either the
// original error, or the result of a Wrap or Mask call.
//
// Cause is the usual way to diagnose errors that may have been wrapped by
// the other errors functions.
func Cause(err error) error {
	var diag error
	if err, ok := err.(causer); ok {
		diag = err.Cause()
	}
	if diag != nil {
		return diag
	}
	return err
}

type causer interface {
	Cause() error
}

type wrapper interface {
	// Message returns the top level error message,
	// not including the message from the Previous
	// error.
	Message() string

	// Underlying returns the Previous error, or nil
	// if there is none.
	Underlying() error
}

type locationer interface {
	Location() (string, int)
}

var (
	_ wrapper    = (*Err)(nil)
	_ locationer = (*Err)(nil)
	_ causer     = (*Err)(nil)
)

// Details returns information about the stack of errors wrapped by err, in
// the format:
//
// 	[{filename:99: error one} {otherfile:55: cause of error one}]
//
// This is a terse alternative to ErrorStack as it returns a single line.
func Details(err error) string {
	if err == nil {
		return "[]"
	}
	var s []byte
	s = append(s, '[')
	for {
		s = append(s, '{')
		if err, ok := err.(locationer); ok {
			file, line := err.Location()
			if file != "" {
				s = append(s, fmt.Sprintf("%s:%d", file, line)...)
				s = append(s, ": "...)
			}
		}
		if cerr, ok := err.(wrapper); ok {
			s = append(s, cerr.Message()...)
			err = cerr.Underlying()
		} else {
			s = append(s, err.Error()...)
			err = nil
		}
		s = append(s, '}')
		if err == nil {
			break
		}
		s = append(s, ' ')
	}
	s = append(s, ']')
	return string(s)
}

// ErrorStack returns a string representation of the annotated error. If the
// error passed as the parameter is not an annotated error, the result is
// simply the result of the Error() method on that error.
//
// If the error is an annotated error, a multi-line string is returned where
// each line represents one entry in the annotation stack. The full filename
// from the call stack is used in the output.
//
//     first error
//     github.com/juju/errors/annotation_test.go:193:
//     github.com/juju/errors/annotation_test.go:194: annotation
//     github.com/juju/errors/annotation_test.go:195:
//     github.com/juju/errors/annotation_test.go:196: more context
//     github.com/juju/errors/annotation_test.go:197:
func ErrorStack(err error) string {
	return strings.Join(errorStack(err), "\n")
}

func errorStack(err error) []string {
	if err == nil {
		return nil
	}

	// We want the first error first
	var lines []string
	for {
		var buff []byte
		if err, ok := err.(locationer); ok {
			file, line := err.Location()
			// Strip off the leading GOPATH/src path elements.
			file = trimGoPath(file)
			if file != "" {
				buff = append(buff, fmt.Sprintf("%s:%d", file, line)...)
				buff = append(buff, ": "...)
			}
		}
		if cerr, ok := err.(wrapper); ok {
			message := cerr.Message()
			buff = append(buff, message...)
			// If there is a cause for this error, and it is different to the cause
			// of the underlying error, then output the error string in the stack trace.
			var cause error
			if err1, ok := err.(causer); ok {
				cause = err1.Cause()
			}
			err = cerr.Underlying()
			if cause != nil && !sameError(Cause(err), cause) {
				if message != "" {
					buff = append(buff, ": "...)
				}
				buff = append(buff, cause.Error()...)
			}
		} else {
			buff = append(buff, err.Error()...)
			err = nil
		}
		lines = append(lines, string(buff))
		if err == nil {
			break
		}
	}
	// reverse the lines to get the original error, which was at the end of
	// the list, back to the start.
	var result []string
	for i := len(lines); i > 0; i-- {
		result = append(result, lines[i-1])
	}
	return result
}
