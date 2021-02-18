// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"time"
)

const (
	metaMediaType = "application/vnd.git-lfs+json"
)

// BatchResponse contains multiple object metadata Representation structures
// for use with the batch API.
type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*Representation `json:"objects"`
}

// Representation is object metadata as seen by clients of the lfs server.
type Representation struct {
	Oid     string           `json:"oid"`
	Size    int64            `json:"size"`
	Actions map[string]*link `json:"actions"`
	Error   *ObjectError     `json:"error,omitempty"`
}

// ObjectError defines the JSON structure returned to the client in case of an error
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// link provides a structure used to build a hypermedia representation of an HTTP link.
type link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

type Pointer struct {
	Oid  string `json:"oid"`
	Size int64  `json:"size"`
}

// BatchVars contains multiple RequestVars processed in one batch operation.
// https://github.com/git-lfs/git-lfs/blob/master/docs/api/batch.md
type BatchVars struct {
	Transfers []string       `json:"transfers,omitempty"`
	Operation string         `json:"operation"`
	Objects   []*RequestVars `json:"objects"`
}

// TODO replace BatchVars in Server
type BatchRequest struct {
	Operation string       `json:"operation"`
	Transfers []string     `json:"transfers,omitempty"`
	Ref       *Reference   `json:"ref,omitempty"`
	Objects   []*LfsObject `json:"objects"`
}

type Reference struct {
	Name string `json:"name"`
}

type LfsObject struct {
	Oid  string `json:"oid"`
	Size int64  `json:"size"`
}