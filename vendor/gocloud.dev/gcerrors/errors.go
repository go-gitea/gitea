// Copyright 2019 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gcerrors provides support for getting error codes from
// errors returned by Go CDK APIs.
package gcerrors

import (
	"context"

	"gocloud.dev/internal/gcerr"
	"golang.org/x/xerrors"
)

// An ErrorCode describes the error's category. Programs should act upon an error's
// code, not its message.
type ErrorCode = gcerr.ErrorCode

const (
	// Returned by the Code function on a nil error. It is not a valid
	// code for an error.
	OK ErrorCode = gcerr.OK

	// The error could not be categorized.
	Unknown ErrorCode = gcerr.Unknown

	// The resource was not found.
	NotFound ErrorCode = gcerr.NotFound

	// The resource exists, but it should not.
	AlreadyExists ErrorCode = gcerr.AlreadyExists

	// A value given to a Go CDK API is incorrect.
	InvalidArgument ErrorCode = gcerr.InvalidArgument

	// Something unexpected happened. Internal errors always indicate
	// bugs in the Go CDK (or possibly the underlying service).
	Internal ErrorCode = gcerr.Internal

	// The feature is not implemented.
	Unimplemented ErrorCode = gcerr.Unimplemented

	// The system was in the wrong state.
	FailedPrecondition ErrorCode = gcerr.FailedPrecondition

	// The caller does not have permission to execute the specified operation.
	PermissionDenied ErrorCode = gcerr.PermissionDenied

	// Some resource has been exhausted, typically because a service resource limit
	// has been reached.
	ResourceExhausted ErrorCode = gcerr.ResourceExhausted

	// The operation was canceled.
	Canceled ErrorCode = gcerr.Canceled

	// The operation timed out.
	DeadlineExceeded ErrorCode = gcerr.DeadlineExceeded
)

// Code returns the ErrorCode of err if it, or some error it wraps, is an *Error.
// If err is context.Canceled or context.DeadlineExceeded, or wraps one of those errors,
// it returns the Canceled or DeadlineExceeded codes, respectively.
// If err is nil, it returns the special code OK.
// Otherwise, it returns Unknown.
func Code(err error) ErrorCode {
	if err == nil {
		return OK
	}
	var e *gcerr.Error
	if xerrors.As(err, &e) {
		return e.Code
	}
	if xerrors.Is(err, context.Canceled) {
		return Canceled
	}
	if xerrors.Is(err, context.DeadlineExceeded) {
		return DeadlineExceeded
	}
	return Unknown
}
