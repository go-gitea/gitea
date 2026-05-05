// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import "code.gitea.io/gitea/modules/json"

const (
	bulkActionIndex  = "index"
	bulkActionDelete = "delete"
)

// BulkOp is a single write inside a Bulk call. Construct with IndexOp or DeleteOp.
type BulkOp struct {
	action string
	id     string
	doc    any
}

// IndexOp builds a bulk index operation.
func IndexOp(id string, doc any) BulkOp {
	return BulkOp{action: bulkActionIndex, id: id, doc: doc}
}

// DeleteOp builds a bulk delete operation.
func DeleteOp(id string) BulkOp {
	return BulkOp{action: bulkActionDelete, id: id}
}

// SortField is one entry of the search sort array.
type SortField struct {
	Field string
	Desc  bool
}

func (s SortField) source() map[string]any {
	order := "asc"
	if s.Desc {
		order = "desc"
	}
	return map[string]any{s.Field: map[string]any{"order": order}}
}

// SearchRequest captures everything Gitea sends to the _search endpoint.
// Aggregations and Highlight are raw ES JSON bodies — callers write them as
// map[string]any since each has exactly one call site with a fixed shape.
type SearchRequest struct {
	Query        Query
	Sort         []SortField
	From         int
	Size         int
	TrackTotal   bool
	Aggregations map[string]any
	Highlight    map[string]any
}

// SearchHit is a single result row.
type SearchHit struct {
	ID        string
	Score     float64
	Source    json.Value
	Highlight map[string][]string
}

// AggBucket is a terms-aggregation bucket.
type AggBucket struct {
	Key      any
	DocCount int64
}

// SearchResponse is Gitea's decoded view of the search reply.
type SearchResponse struct {
	Total        int64
	Hits         []SearchHit
	Aggregations map[string][]AggBucket
}
