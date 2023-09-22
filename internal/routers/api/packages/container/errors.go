// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"net/http"
)

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes
var (
	errBlobUnknown         = &namedError{Code: "BLOB_UNKNOWN", StatusCode: http.StatusNotFound}
	errBlobUploadInvalid   = &namedError{Code: "BLOB_UPLOAD_INVALID", StatusCode: http.StatusBadRequest}
	errBlobUploadUnknown   = &namedError{Code: "BLOB_UPLOAD_UNKNOWN", StatusCode: http.StatusNotFound}
	errDigestInvalid       = &namedError{Code: "DIGEST_INVALID", StatusCode: http.StatusBadRequest}
	errManifestBlobUnknown = &namedError{Code: "MANIFEST_BLOB_UNKNOWN", StatusCode: http.StatusNotFound}
	errManifestInvalid     = &namedError{Code: "MANIFEST_INVALID", StatusCode: http.StatusBadRequest}
	errManifestUnknown     = &namedError{Code: "MANIFEST_UNKNOWN", StatusCode: http.StatusNotFound}
	errNameInvalid         = &namedError{Code: "NAME_INVALID", StatusCode: http.StatusBadRequest}
	errNameUnknown         = &namedError{Code: "NAME_UNKNOWN", StatusCode: http.StatusNotFound}
	errSizeInvalid         = &namedError{Code: "SIZE_INVALID", StatusCode: http.StatusBadRequest}
	errUnauthorized        = &namedError{Code: "UNAUTHORIZED", StatusCode: http.StatusUnauthorized}
	errUnsupported         = &namedError{Code: "UNSUPPORTED", StatusCode: http.StatusNotImplemented}
)

type namedError struct {
	Code       string
	StatusCode int
	Message    string
}

func (e *namedError) Error() string {
	return e.Message
}

// WithMessage creates a new instance of the error with a different message
func (e *namedError) WithMessage(message string) *namedError {
	return &namedError{
		Code:       e.Code,
		StatusCode: e.StatusCode,
		Message:    message,
	}
}

// WithStatusCode creates a new instance of the error with a different status code
func (e *namedError) WithStatusCode(statusCode int) *namedError {
	return &namedError{
		Code:       e.Code,
		StatusCode: statusCode,
		Message:    e.Message,
	}
}
