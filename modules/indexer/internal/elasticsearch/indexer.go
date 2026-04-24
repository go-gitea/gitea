// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/json"

	"github.com/elastic/go-elasticsearch/v8"
)

var _ internal.Indexer = &Indexer{}

// Indexer is Gitea's narrow wrapper around an Elasticsearch/OpenSearch cluster.
type Indexer struct {
	client *elasticsearch.Client

	url       string
	indexName string
	version   int
	mapping   string
}

// NewIndexer builds an Indexer. The connection is opened by Init.
func NewIndexer(url, indexName string, version int, mapping string) *Indexer {
	return &Indexer{
		url:       url,
		indexName: indexName,
		version:   version,
		mapping:   mapping,
	}
}

// Init connects and creates the versioned index if missing, returning true if it already existed.
func (i *Indexer) Init(ctx context.Context) (bool, error) {
	if i == nil {
		return false, errors.New("cannot init nil indexer")
	}
	if i.client != nil {
		return false, errors.New("indexer is already initialized")
	}

	client, err := i.newClient()
	if err != nil {
		return false, err
	}
	i.client = client

	exists, err := i.indexExists(ctx, i.VersionedIndexName())
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}

	if err := i.createIndex(ctx); err != nil {
		return false, err
	}

	return false, nil
}

// Ping returns an error when the cluster is unusable (status != green/yellow).
func (i *Indexer) Ping(ctx context.Context) error {
	if i == nil {
		return errors.New("cannot ping nil indexer")
	}
	if i.client == nil {
		return errors.New("indexer is not initialized")
	}

	res, err := i.client.Cluster.Health(
		i.client.Cluster.Health.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("cluster health: %s", res.String())
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	// Healthy = green; usable = yellow. Red is unusable.
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html
	if body.Status != "green" && body.Status != "yellow" {
		return fmt.Errorf("status of elasticsearch cluster is %s", body.Status)
	}
	return nil
}

// Close drops the client reference; idle HTTP connections are closed by the GC.
func (i *Indexer) Close() {
	if i == nil {
		return
	}
	i.client = nil
}

// Bulk submits index/delete ops. Returns the first item-level failure, if any.
func (i *Indexer) Bulk(ctx context.Context, ops []BulkOp) error {
	if len(ops) == 0 {
		return nil
	}

	index := i.VersionedIndexName()
	var buf bytes.Buffer
	for _, op := range ops {
		meta := map[string]any{string(op.action): map[string]any{"_index": index, "_id": op.id}}
		if err := writeJSONLine(&buf, meta); err != nil {
			return err
		}
		if op.action == bulkActionIndex {
			if err := writeJSONLine(&buf, op.doc); err != nil {
				return err
			}
		}
	}

	res, err := i.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		i.client.Bulk.WithContext(ctx),
		i.client.Bulk.WithIndex(index),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("bulk: %s", res.String())
	}

	var body struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int        `json:"status"`
			Error  json.Value `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	if body.Errors {
		for _, item := range body.Items {
			for _, result := range item {
				if len(result.Error) > 0 {
					return fmt.Errorf("bulk item failed (status %d): %s", result.Status, string(result.Error))
				}
			}
		}
	}
	return nil
}

// Index writes a single document.
func (i *Indexer) Index(ctx context.Context, id string, doc any) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	res, err := i.client.Index(
		i.VersionedIndexName(),
		bytes.NewReader(body),
		i.client.Index.WithContext(ctx),
		i.client.Index.WithDocumentID(id),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index: %s", res.String())
	}
	return nil
}

// Delete removes a single document by id. Missing ids are not an error.
func (i *Indexer) Delete(ctx context.Context, id string) error {
	res, err := i.client.Delete(
		i.VersionedIndexName(),
		id,
		i.client.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete: %s", res.String())
	}
	return nil
}

// DeleteByQuery removes every document matching the query.
func (i *Indexer) DeleteByQuery(ctx context.Context, query Query) error {
	body, err := json.Marshal(map[string]any{"query": query.querySource()})
	if err != nil {
		return err
	}
	res, err := i.client.DeleteByQuery(
		[]string{i.VersionedIndexName()},
		bytes.NewReader(body),
		i.client.DeleteByQuery.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("delete_by_query: %s", res.String())
	}
	return nil
}

// Refresh forces a refresh so recent writes are searchable.
func (i *Indexer) Refresh(ctx context.Context) error {
	res, err := i.client.Indices.Refresh(
		i.client.Indices.Refresh.WithContext(ctx),
		i.client.Indices.Refresh.WithIndex(i.VersionedIndexName()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("refresh: %s", res.String())
	}
	return nil
}

// Search runs a search request and decodes the reply.
func (i *Indexer) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	body := map[string]any{}
	if req.Query != nil {
		body["query"] = req.Query.querySource()
	}
	if len(req.Sort) > 0 {
		sorts := make([]map[string]any, len(req.Sort))
		for idx, s := range req.Sort {
			sorts[idx] = s.source()
		}
		body["sort"] = sorts
	}
	if req.From > 0 {
		body["from"] = req.From
	}
	body["size"] = req.Size
	if len(req.Aggregations) > 0 {
		body["aggs"] = req.Aggregations
	}
	if len(req.Highlight) > 0 {
		body["highlight"] = req.Highlight
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := i.client.Search(
		i.client.Search.WithContext(ctx),
		i.client.Search.WithIndex(i.VersionedIndexName()),
		i.client.Search.WithBody(bytes.NewReader(payload)),
		i.client.Search.WithTrackTotalHits(req.TrackTotal),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search: %s", res.String())
	}

	return decodeSearchResponse(res.Body)
}

func (i *Indexer) indexExists(ctx context.Context, name string) (bool, error) {
	res, err := i.client.Indices.Exists(
		[]string{name},
		i.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return true, nil
	}
	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("indices.exists: %s", res.String())
}

func (i *Indexer) createIndex(ctx context.Context) error {
	res, err := i.client.Indices.Create(
		i.VersionedIndexName(),
		i.client.Indices.Create.WithContext(ctx),
		i.client.Indices.Create.WithBody(strings.NewReader(i.mapping)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("create index %s: %s", i.VersionedIndexName(), res.String())
	}

	var body struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	if !body.Acknowledged {
		return fmt.Errorf("create index %s not acknowledged", i.VersionedIndexName())
	}

	i.checkOldIndexes(ctx)
	return nil
}

func writeJSONLine(buf *bytes.Buffer, v any) error {
	enc, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf.Write(enc)
	buf.WriteByte('\n')
	return nil
}

func decodeSearchResponse(r io.Reader) (*SearchResponse, error) {
	var raw struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    json.Value          `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]struct {
			Buckets []struct {
				Key      any   `json:"key"`
				DocCount int64 `json:"doc_count"`
			} `json:"buckets"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	resp := &SearchResponse{
		Total: raw.Hits.Total.Value,
		Hits:  make([]SearchHit, 0, len(raw.Hits.Hits)),
	}
	for _, h := range raw.Hits.Hits {
		resp.Hits = append(resp.Hits, SearchHit{
			ID:        h.ID,
			Score:     h.Score,
			Source:    h.Source,
			Highlight: h.Highlight,
		})
	}
	if len(raw.Aggregations) > 0 {
		resp.Aggregations = make(map[string][]AggBucket, len(raw.Aggregations))
		for name, agg := range raw.Aggregations {
			buckets := make([]AggBucket, len(agg.Buckets))
			for idx, b := range agg.Buckets {
				buckets[idx] = AggBucket{Key: b.Key, DocCount: b.DocCount}
			}
			resp.Aggregations[name] = buckets
		}
	}
	return resp, nil
}
