// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import "errors"

// PercolatorQuery can be used to match queries stored in an index.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-percolate-query.html
type PercolatorQuery struct {
	field                     string
	name                      string
	documentType              string // deprecated
	documents                 []interface{}
	indexedDocumentIndex      string
	indexedDocumentType       string
	indexedDocumentId         string
	indexedDocumentRouting    string
	indexedDocumentPreference string
	indexedDocumentVersion    *int64
}

// NewPercolatorQuery creates and initializes a new Percolator query.
func NewPercolatorQuery() *PercolatorQuery {
	return &PercolatorQuery{}
}

func (q *PercolatorQuery) Field(field string) *PercolatorQuery {
	q.field = field
	return q
}

// Name used for identification purposes in "_percolator_document_slot" response
// field when multiple percolate queries have been specified in the main query.
func (q *PercolatorQuery) Name(name string) *PercolatorQuery {
	q.name = name
	return q
}

// Deprecated: DocumentType is deprecated as of 6.0.
func (q *PercolatorQuery) DocumentType(typ string) *PercolatorQuery {
	q.documentType = typ
	return q
}

func (q *PercolatorQuery) Document(docs ...interface{}) *PercolatorQuery {
	q.documents = append(q.documents, docs...)
	return q
}

func (q *PercolatorQuery) IndexedDocumentIndex(index string) *PercolatorQuery {
	q.indexedDocumentIndex = index
	return q
}

func (q *PercolatorQuery) IndexedDocumentType(typ string) *PercolatorQuery {
	q.indexedDocumentType = typ
	return q
}

func (q *PercolatorQuery) IndexedDocumentId(id string) *PercolatorQuery {
	q.indexedDocumentId = id
	return q
}

func (q *PercolatorQuery) IndexedDocumentRouting(routing string) *PercolatorQuery {
	q.indexedDocumentRouting = routing
	return q
}

func (q *PercolatorQuery) IndexedDocumentPreference(preference string) *PercolatorQuery {
	q.indexedDocumentPreference = preference
	return q
}

func (q *PercolatorQuery) IndexedDocumentVersion(version int64) *PercolatorQuery {
	q.indexedDocumentVersion = &version
	return q
}

// Source returns JSON for the percolate query.
func (q *PercolatorQuery) Source() (interface{}, error) {
	if len(q.field) == 0 {
		return nil, errors.New("elastic: Field is required in PercolatorQuery")
	}

	// {
	//   "percolate" : { ... }
	// }
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["percolate"] = params
	params["field"] = q.field
	if q.documentType != "" {
		params["document_type"] = q.documentType
	}
	if q.name != "" {
		params["name"] = q.name
	}

	switch len(q.documents) {
	case 0:
	case 1:
		params["document"] = q.documents[0]
	default:
		params["documents"] = q.documents
	}

	if s := q.indexedDocumentIndex; s != "" {
		params["index"] = s
	}
	if s := q.indexedDocumentType; s != "" {
		params["type"] = s
	}
	if s := q.indexedDocumentId; s != "" {
		params["id"] = s
	}
	if s := q.indexedDocumentRouting; s != "" {
		params["routing"] = s
	}
	if s := q.indexedDocumentPreference; s != "" {
		params["preference"] = s
	}
	if v := q.indexedDocumentVersion; v != nil {
		params["version"] = *v
	}
	return source, nil
}
