// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/util"
)

const (
	// MediaType contains the media type for LFS server requests
	MediaType = "application/vnd.git-lfs+json"
	// AcceptHeader Some LFS servers offer content with other types, so fallback to '*/*' if application/vnd.git-lfs+json cannot be served
	AcceptHeader = "application/vnd.git-lfs+json;q=0.9, */*;q=0.8"
	// UserAgentHeader Add User-Agent for gitea's self-implemented lfs client,
	// and the version is consistent with the latest version of git lfs can be avoided incompatibilities.
	// Some lfs servers will check this
	UserAgentHeader = "git-lfs/3.6.0 (Gitea)"
)

// BatchRequest contains multiple requests processed in one batch operation.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#requests
type BatchRequest struct {
	Operation string     `json:"operation"`
	Transfers []string   `json:"transfers,omitempty"`
	Ref       *Reference `json:"ref,omitempty"`
	Objects   []Pointer  `json:"objects"`
}

// Reference contains a git reference.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#ref-property
type Reference struct {
	Name string `json:"name"`
}

// Pointer contains LFS pointer data
type Pointer struct {
	Oid  string `json:"oid" xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size int64  `json:"size" xorm:"NOT NULL"`
}

// BatchResponse contains multiple object metadata Representation structures
// for use with the batch API.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#successful-responses
type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*ObjectResponse `json:"objects"`
}

// ObjectResponse is object metadata as seen by clients of the LFS server.
type ObjectResponse struct {
	Pointer
	Actions map[string]*Link `json:"actions,omitempty"`
	Links   map[string]*Link `json:"_links,omitempty"`
	Error   *ObjectError     `json:"error,omitempty"`
}

// Link provides a structure with information about how to access a object.
type Link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// ObjectError defines the JSON structure returned to the client in case of an error.
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	// See https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#successful-responses
	// LFS object error codes should match HTTP status codes where possible:
	//   404 - The object does not exist on the server.
	//   409 - The specified hash algorithm disagrees with the server's acceptable options.
	//   410 - The object was removed by the owner.
	//   422 - Validation error.

	ErrObjectNotExist     = util.ErrNotExist // the object does not exist on the server
	ErrObjectHashMismatch = errors.New("the specified hash algorithm disagrees with the server's acceptable options")
	ErrObjectRemoved      = errors.New("the object was removed by the owner")
	ErrObjectValidation   = errors.New("validation error")
)

func (e *ObjectError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *ObjectError) Unwrap() error {
	switch e.Code {
	case 404:
		return ErrObjectNotExist
	case 409:
		return ErrObjectHashMismatch
	case 410:
		return ErrObjectRemoved
	case 422:
		return ErrObjectValidation
	default:
		return errors.New(e.Message)
	}
}

// PointerBlob associates a Git blob with a Pointer.
type PointerBlob struct {
	Hash string
	Pointer
}

// ErrorResponse describes the error to the client.
type ErrorResponse struct {
	Message          string
	DocumentationURL string `json:"documentation_url,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
}
