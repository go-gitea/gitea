// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

//go:generate easyjson bulk_index_request.go

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BulkIndexRequest is a request to add a document to Elasticsearch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-bulk.html
// for details.
type BulkIndexRequest struct {
	BulkableRequest
	index           string
	typ             string
	id              string
	opType          string
	routing         string
	parent          string
	version         int64  // default is MATCH_ANY
	versionType     string // default is "internal"
	doc             interface{}
	pipeline        string
	retryOnConflict *int
	ttl             string

	source []string

	useEasyJSON bool
}

//easyjson:json
type bulkIndexRequestCommand map[string]bulkIndexRequestCommandOp

//easyjson:json
type bulkIndexRequestCommandOp struct {
	Id              string `json:"_id,omitempty"`
	Index           string `json:"_index,omitempty"`
	TTL             string `json:"_ttl,omitempty"`
	Type            string `json:"_type,omitempty"`
	Parent          string `json:"_parent,omitempty"`
	RetryOnConflict *int   `json:"_retry_on_conflict,omitempty"`
	Routing         string `json:"_routing,omitempty"`
	Version         int64  `json:"_version,omitempty"`
	VersionType     string `json:"_version_type,omitempty"`
	Pipeline        string `json:"pipeline,omitempty"`
}

// NewBulkIndexRequest returns a new BulkIndexRequest.
// The operation type is "index" by default.
func NewBulkIndexRequest() *BulkIndexRequest {
	return &BulkIndexRequest{
		opType: "index",
	}
}

// UseEasyJSON is an experimental setting that enables serialization
// with github.com/mailru/easyjson, which should in faster serialization
// time and less allocations, but removed compatibility with encoding/json,
// usage of unsafe etc. See https://github.com/mailru/easyjson#issues-notes-and-limitations
// for details. This setting is disabled by default.
func (r *BulkIndexRequest) UseEasyJSON(enable bool) *BulkIndexRequest {
	r.useEasyJSON = enable
	return r
}

// Index specifies the Elasticsearch index to use for this index request.
// If unspecified, the index set on the BulkService will be used.
func (r *BulkIndexRequest) Index(index string) *BulkIndexRequest {
	r.index = index
	r.source = nil
	return r
}

// Type specifies the Elasticsearch type to use for this index request.
// If unspecified, the type set on the BulkService will be used.
func (r *BulkIndexRequest) Type(typ string) *BulkIndexRequest {
	r.typ = typ
	r.source = nil
	return r
}

// Id specifies the identifier of the document to index.
func (r *BulkIndexRequest) Id(id string) *BulkIndexRequest {
	r.id = id
	r.source = nil
	return r
}

// OpType specifies if this request should follow create-only or upsert
// behavior. This follows the OpType of the standard document index API.
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-index_.html#operation-type
// for details.
func (r *BulkIndexRequest) OpType(opType string) *BulkIndexRequest {
	r.opType = opType
	r.source = nil
	return r
}

// Routing specifies a routing value for the request.
func (r *BulkIndexRequest) Routing(routing string) *BulkIndexRequest {
	r.routing = routing
	r.source = nil
	return r
}

// Parent specifies the identifier of the parent document (if available).
func (r *BulkIndexRequest) Parent(parent string) *BulkIndexRequest {
	r.parent = parent
	r.source = nil
	return r
}

// Version indicates the version of the document as part of an optimistic
// concurrency model.
func (r *BulkIndexRequest) Version(version int64) *BulkIndexRequest {
	r.version = version
	r.source = nil
	return r
}

// VersionType specifies how versions are created. It can be e.g. internal,
// external, external_gte, or force.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-index_.html#index-versioning
// for details.
func (r *BulkIndexRequest) VersionType(versionType string) *BulkIndexRequest {
	r.versionType = versionType
	r.source = nil
	return r
}

// Doc specifies the document to index.
func (r *BulkIndexRequest) Doc(doc interface{}) *BulkIndexRequest {
	r.doc = doc
	r.source = nil
	return r
}

// RetryOnConflict specifies how often to retry in case of a version conflict.
func (r *BulkIndexRequest) RetryOnConflict(retryOnConflict int) *BulkIndexRequest {
	r.retryOnConflict = &retryOnConflict
	r.source = nil
	return r
}

// TTL is an expiration time for the document.
func (r *BulkIndexRequest) TTL(ttl string) *BulkIndexRequest {
	r.ttl = ttl
	r.source = nil
	return r
}

// Pipeline to use while processing the request.
func (r *BulkIndexRequest) Pipeline(pipeline string) *BulkIndexRequest {
	r.pipeline = pipeline
	r.source = nil
	return r
}

// String returns the on-wire representation of the index request,
// concatenated as a single string.
func (r *BulkIndexRequest) String() string {
	lines, err := r.Source()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.Join(lines, "\n")
}

// Source returns the on-wire representation of the index request,
// split into an action-and-meta-data line and an (optional) source line.
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/docs-bulk.html
// for details.
func (r *BulkIndexRequest) Source() ([]string, error) {
	// { "index" : { "_index" : "test", "_type" : "type1", "_id" : "1" } }
	// { "field1" : "value1" }

	if r.source != nil {
		return r.source, nil
	}

	lines := make([]string, 2)

	// "index" ...
	indexCommand := bulkIndexRequestCommandOp{
		Index:           r.index,
		Type:            r.typ,
		Id:              r.id,
		Routing:         r.routing,
		Parent:          r.parent,
		Version:         r.version,
		VersionType:     r.versionType,
		RetryOnConflict: r.retryOnConflict,
		TTL:             r.ttl,
		Pipeline:        r.pipeline,
	}
	command := bulkIndexRequestCommand{
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
