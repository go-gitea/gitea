// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"net/http"
)

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes
var (
	errBlobUnknown         = &Error{Code: "BLOB_UNKNOWN", StatusCode: http.StatusNotFound}
	errBlobUploadInvalid   = &Error{Code: "BLOB_UPLOAD_INVALID", StatusCode: http.StatusBadRequest}
	errBlobUploadUnknown   = &Error{Code: "BLOB_UPLOAD_UNKNOWN", StatusCode: http.StatusNotFound}
	errDigestInvalid       = &Error{Code: "DIGEST_INVALID", StatusCode: http.StatusBadRequest}
	errManifestBlobUnknown = &Error{Code: "MANIFEST_BLOB_UNKNOWN", StatusCode: http.StatusNotFound}
	errManifestInvalid     = &Error{Code: "MANIFEST_INVALID", StatusCode: http.StatusBadRequest}
	errManifestUnknown     = &Error{Code: "MANIFEST_UNKNOWN", StatusCode: http.StatusNotFound}
	errNameInvalid         = &Error{Code: "NAME_INVALID", StatusCode: http.StatusBadRequest}
	errNameUnknown         = &Error{Code: "NAME_UNKNOWN", StatusCode: http.StatusNotFound}
	errSizeInvalid         = &Error{Code: "SIZE_INVALID", StatusCode: http.StatusBadRequest}
	errUnauthorized        = &Error{Code: "UNAUTHORIZED", StatusCode: http.StatusUnauthorized}
	errUnsupported         = &Error{Code: "UNSUPPORTED", StatusCode: http.StatusNotImplemented}
)

type Errors struct {
	Errors []Error `json:"errors"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int
}

func (e *Error) Error() string {
	return e.Message
}

// WithMessage creates a new instance of the error with a different message
func (e *Error) WithMessage(message string) *Error {
	return &Error{
		Code:       e.Code,
		StatusCode: e.StatusCode,
		Message:    message,
	}
}

// WithStatusCode creates a new instance of the error with a different status code
func (e *Error) WithStatusCode(statusCode int) *Error {
	return &Error{
		Code:       e.Code,
		StatusCode: statusCode,
		Message:    e.Message,
	}
}
