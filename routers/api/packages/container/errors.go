// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"net/http"
)

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes
var (
	errBlobUnknown             = &namedError{Code: "BLOB_UNKNOWN", StatusCode: http.StatusNotFound, Message: "blob unknown to registry"}
	errBlobUploadInvalid       = &namedError{Code: "BLOB_UPLOAD_INVALID", StatusCode: http.StatusBadRequest, Message: "blob upload invalid"}
	errBlobUploadUnknown       = &namedError{Code: "BLOB_UPLOAD_UNKNOWN", StatusCode: http.StatusNotFound, Message: "blob upload unknown to registry"}
	errDigestInvalid           = &namedError{Code: "DIGEST_INVALID", StatusCode: http.StatusBadRequest, Message: "provided digest did not match uploaded content"}
	errManifestBlobUnknown     = &namedError{Code: "MANIFEST_BLOB_UNKNOWN", StatusCode: http.StatusNotFound, Message: "blob unknown to registry"}
	errManifestInvalid         = &namedError{Code: "MANIFEST_INVALID", StatusCode: http.StatusBadRequest, Message: "manifest invalid"}
	errManifestUnknown         = &namedError{Code: "MANIFEST_UNKNOWN", StatusCode: http.StatusNotFound, Message: "manifest unknown"}
	errManifestUnverified      = &namedError{Code: "MANIFEST_UNVERIFIED", StatusCode: http.StatusBadRequest, Message: "manifest failed signature verification"} //nolint:unused
	errNameInvalid             = &namedError{Code: "NAME_INVALID", StatusCode: http.StatusBadRequest, Message: "invalid repository name"}
	errNameUnknown             = &namedError{Code: "NAME_UNKNOWN", StatusCode: http.StatusNotFound, Message: "repository name not known to registry"}
	errPaginationNumberInvalid = &namedError{Code: "PAGINATION_NUMBER_INVALID", StatusCode: http.StatusBadRequest, Message: "invalid number of results requested"} //nolint:unused
	errRangeInvalid            = &namedError{Code: "RANGE_INVALID", StatusCode: http.StatusBadRequest, Message: "invalid content range"}                           //nolint:unused
	errSizeInvalid             = &namedError{Code: "SIZE_INVALID", StatusCode: http.StatusBadRequest, Message: "provided length did not match content length"}
	errTagInvalid              = &namedError{Code: "TAG_INVALID", StatusCode: http.StatusBadRequest, Message: "manifest tag did not match URI"}
	errUnauthorized            = &namedError{Code: "UNAUTHORIZED", StatusCode: http.StatusUnauthorized, Message: "authentication required"}
	errDenied                  = &namedError{Code: "DENIED", StatusCode: http.StatusForbidden, Message: "requested access to the resource is denied"} //nolint:unused
	errUnsupported             = &namedError{Code: "UNSUPPORTED", StatusCode: http.StatusMethodNotAllowed, Message: "The operation is unsupported"}
)

type namedError struct {
	Code       string
	StatusCode int
	Message    string
	Detail     any
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

// WithDetail creates a new instance of the error with detail
func (e *namedError) WithDetail(detail interface{}) *namedError {
	return &namedError{
		Code:       e.Code,
		StatusCode: e.StatusCode,
		Message:    e.Message,
		Detail:     detail,
	}
}
