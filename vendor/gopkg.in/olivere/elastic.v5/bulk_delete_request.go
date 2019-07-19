// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

//go:generate easyjson bulk_delete_request.go

import (
	"encoding/json"
	"fmt"
	"strings"
)

// -- Bulk delete request --

// BulkDeleteRequest is a request to remove a document from Elasticsearch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-bulk.html
// for details.
type BulkDeleteRequest struct {
	BulkableRequest
	index       string
	typ         string
	id          string
	parent      string
	routing     string
	version     int64  // default is MATCH_ANY
	versionType string // default is "internal"

	source []string

	useEasyJSON bool
}

//easyjson:json
type bulkDeleteRequestCommand map[string]bulkDeleteRequestCommandOp

//easyjson:json
type bulkDeleteRequestCommandOp struct {
	Id          string `json:"_id,omitempty"`
	Index       string `json:"_index,omitempty"`
	Parent      string `json:"_parent,omitempty"`
	Routing     string `json:"_routing,omitempty"`
	Type        string `json:"_type,omitempty"`
	Version     int64  `json:"_version,omitempty"`
	VersionType string `json:"_version_type,omitempty"`
}

// NewBulkDeleteRequest returns a new BulkDeleteRequest.
func NewBulkDeleteRequest() *BulkDeleteRequest {
	return &BulkDeleteRequest{}
}

// UseEasyJSON is an experimental setting that enables serialization
// with github.com/mailru/easyjson, which should in faster serialization
// time and less allocations, but removed compatibility with encoding/json,
// usage of unsafe etc. See https://github.com/mailru/easyjson#issues-notes-and-limitations
// for details. This setting is disabled by default.
func (r *BulkDeleteRequest) UseEasyJSON(enable bool) *BulkDeleteRequest {
	r.useEasyJSON = enable
	return r
}

// Index specifies the Elasticsearch index to use for this delete request.
// If unspecified, the index set on the BulkService will be used.
func (r *BulkDeleteRequest) Index(index string) *BulkDeleteRequest {
	r.index = index
	r.source = nil
	return r
}

// Type specifies the Elasticsearch type to use for this delete request.
// If unspecified, the type set on the BulkService will be used.
func (r *BulkDeleteRequest) Type(typ string) *BulkDeleteRequest {
	r.typ = typ
	r.source = nil
	return r
}

// Id specifies the identifier of the document to delete.
func (r *BulkDeleteRequest) Id(id string) *BulkDeleteRequest {
	r.id = id
	r.source = nil
	return r
}

// Parent specifies the parent of the request, which is used in parent/child
// mappings.
func (r *BulkDeleteRequest) Parent(parent string) *BulkDeleteRequest {
	r.parent = parent
	r.source = nil
	return r
}

// Routing specifies a routing value for the request.
func (r *BulkDeleteRequest) Routing(routing string) *BulkDeleteRequest {
	r.routing = routing
	r.source = nil
	return r
}

// Version indicates the version to be deleted as part of an optimistic
// concurrency model.
func (r *BulkDeleteRequest) Version(version int64) *BulkDeleteRequest {
	r.version = version
	r.source = nil
	return r
}

// VersionType can be "internal" (default), "external", "external_gte",
// "external_gt", or "force".
func (r *BulkDeleteRequest) VersionType(versionType string) *BulkDeleteRequest {
	r.versionType = versionType
	r.source = nil
	return r
}

// String returns the on-wire representation of the delete request,
// concatenated as a single string.
func (r *BulkDeleteRequest) String() string {
	lines, err := r.Source()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.Join(lines, "\n")
}

// Source returns the on-wire representation of the delete request,
// split into an action-and-meta-data line and an (optional) source line.
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-bulk.html
// for details.
func (r *BulkDeleteRequest) Source() ([]string, error) {
	if r.source != nil {
		return r.source, nil
	}
	command := bulkDeleteRequestCommand{
		"delete": bulkDeleteRequestCommandOp{
			Index:       r.index,
			Type:        r.typ,
			Id:          r.id,
			Routing:     r.routing,
			Parent:      r.parent,
			Version:     r.version,
			VersionType: r.versionType,
		},
	}

	var err error
	var body []byte
	if r.useEasyJSON {
		// easyjson
		body, err = command.MarshalJSON()
	} else {
		// encoding/json
		body, err = json.Marshal(command)
	}
	if err != nil {
		return nil, err
	}

	lines := []string{string(body)}
	r.source = lines

	return lines, nil
}
