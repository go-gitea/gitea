// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

//go:generate easyjson bulk_create_request.go

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BulkCreateRequest is a request to add a new document to Elasticsearch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-bulk.html
// for details.
type BulkCreateRequest struct {
	BulkableRequest
	index           string
	typ             string
	id              string
	opType          string
	routing         string
	parent          string
	version         *int64 // default is MATCH_ANY
	versionType     string // default is "internal"
	doc             interface{}
	pipeline        string
	retryOnConflict *int
	ifSeqNo         *int64
	ifPrimaryTerm   *int64

	source []string

	useEasyJSON bool
}

//easyjson:json
type bulkCreateRequestCommand map[string]bulkCreateRequestCommandOp

//easyjson:json
type bulkCreateRequestCommandOp struct {
	Index  string `json:"_index,omitempty"`
	Id     string `json:"_id,omitempty"`
	Type   string `json:"_type,omitempty"`
	Parent string `json:"parent,omitempty"`
	// RetryOnConflict is "_retry_on_conflict" for 6.0 and "retry_on_conflict" for 6.1+.
	RetryOnConflict *int   `json:"retry_on_conflict,omitempty"`
	Routing         string `json:"routing,omitempty"`
	Version         *int64 `json:"version,omitempty"`
	VersionType     string `json:"version_type,omitempty"`
	Pipeline        string `json:"pipeline,omitempty"`
	IfSeqNo         *int64 `json:"if_seq_no,omitempty"`
	IfPrimaryTerm   *int64 `json:"if_primary_term,omitempty"`
}

// NewBulkCreateRequest returns a new BulkCreateRequest.
// The operation type is "create" by default.
func NewBulkCreateRequest() *BulkCreateRequest {
	return &BulkCreateRequest{
		opType: "create",
	}
}

// UseEasyJSON is an experimental setting that enables serialization
// with github.com/mailru/easyjson, which should in faster serialization
// time and less allocations, but removed compatibility with encoding/json,
// usage of unsafe etc. See https://github.com/mailru/easyjson#issues-notes-and-limitations
// for details. This setting is disabled by default.
func (r *BulkCreateRequest) UseEasyJSON(enable bool) *BulkCreateRequest {
	r.useEasyJSON = enable
	return r
}

// Index specifies the Elasticsearch index to use for this create request.
// If unspecified, the index set on the BulkService will be used.
func (r *BulkCreateRequest) Index(index string) *BulkCreateRequest {
	r.index = index
	r.source = nil
	return r
}

// Type specifies the Elasticsearch type to use for this create request.
// If unspecified, the type set on the BulkService will be used.
func (r *BulkCreateRequest) Type(typ string) *BulkCreateRequest {
	r.typ = typ
	r.source = nil
	return r
}

// Id specifies the identifier of the document to create.
func (r *BulkCreateRequest) Id(id string) *BulkCreateRequest {
	r.id = id
	r.source = nil
	return r
}

// Routing specifies a routing value for the request.
func (r *BulkCreateRequest) Routing(routing string) *BulkCreateRequest {
	r.routing = routing
	r.source = nil
	return r
}

// Parent specifies the identifier of the parent document (if available).
func (r *BulkCreateRequest) Parent(parent string) *BulkCreateRequest {
	r.parent = parent
	r.source = nil
	return r
}

// Version indicates the version of the document as part of an optimistic
// concurrency model.
func (r *BulkCreateRequest) Version(version int64) *BulkCreateRequest {
	v := version
	r.version = &v
	r.source = nil
	return r
}

// VersionType specifies how versions are created. It can be e.g. internal,
// external, external_gte, or force.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-index_.html#index-versioning
// for details.
func (r *BulkCreateRequest) VersionType(versionType string) *BulkCreateRequest {
	r.versionType = versionType
	r.source = nil
	return r
}

// Doc specifies the document to create.
func (r *BulkCreateRequest) Doc(doc interface{}) *BulkCreateRequest {
	r.doc = doc
	r.source = nil
	return r
}

// RetryOnConflict specifies how often to retry in case of a version conflict.
func (r *BulkCreateRequest) RetryOnConflict(retryOnConflict int) *BulkCreateRequest {
	r.retryOnConflict = &retryOnConflict
	r.source = nil
	return r
}

// Pipeline to use while processing the request.
func (r *BulkCreateRequest) Pipeline(pipeline string) *BulkCreateRequest {
	r.pipeline = pipeline
	r.source = nil
	return r
}

// IfSeqNo indicates to only perform the create operation if the last
// operation that has changed the document has the specified sequence number.
func (r *BulkCreateRequest) IfSeqNo(ifSeqNo int64) *BulkCreateRequest {
	r.ifSeqNo = &ifSeqNo
	return r
}

// IfPrimaryTerm indicates to only perform the create operation if the
// last operation that has changed the document has the specified primary term.
func (r *BulkCreateRequest) IfPrimaryTerm(ifPrimaryTerm int64) *BulkCreateRequest {
	r.ifPrimaryTerm = &ifPrimaryTerm
	return r
}

// String returns the on-wire representation of the create request,
// concatenated as a single string.
func (r *BulkCreateRequest) String() string {
	lines, err := r.Source()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.Join(lines, "\n")
}

// Source returns the on-wire representation of the create request,
// split into an action-and-meta-data line and an (optional) source line.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-bulk.html
// for details.
func (r *BulkCreateRequest) Source() ([]string, error) {
	// { "create" : { "_index" : "test", "_type" : "type1", "_id" : "1" } }
	// { "field1" : "value1" }

	if r.source != nil {
		return r.source, nil
	}

	lines := make([]string, 2)

	// "index" ...
	indexCommand := bulkCreateRequestCommandOp{
		Index:           r.index,
		Type:            r.typ,
		Id:              r.id,
		Routing:         r.routing,
		Parent:          r.parent,
		Version:         r.version,
		VersionType:     r.versionType,
		RetryOnConflict: r.retryOnConflict,
		Pipeline:        r.pipeline,
		IfSeqNo:         r.ifSeqNo,
		IfPrimaryTerm:   r.ifPrimaryTerm,
	}
	command := bulkCreateRequestCommand{
		r.opType: indexCommand,
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

	lines[0] = string(body)

	// "field1" ...
	if r.doc != nil {
		switch t := r.doc.(type) {
		default:
			body, err := json.Marshal(r.doc)
			if err != nil {
				return nil, err
			}
			lines[1] = string(body)
		case json.RawMessage:
			lines[1] = string(t)
		case *json.RawMessage:
			lines[1] = string(*t)
		case string:
			lines[1] = t
		case *string:
			lines[1] = *t
		}
	} else {
		lines[1] = "{}"
	}

	r.source = lines
	return lines, nil
}
