// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/json"
)

var _ internal.Indexer = &Indexer{}

// Indexer is a narrow wrapper around an Elasticsearch/OpenSearch cluster.
// It targets the REST subset shared by Elasticsearch 7/8/9 and OpenSearch 3.
type Indexer struct {
	client *http.Client
	base   string // base URL with trailing slash, no userinfo
	user   string
	pass   string

	indexName string
	version   int
	mapping   string
}

// NewIndexer builds an Indexer. The connection is opened by Init.
func NewIndexer(rawURL, indexName string, version int, mapping string) *Indexer {
	return &Indexer{
		base:      rawURL,
		indexName: indexName,
		version:   version,
		mapping:   mapping,
	}
}

// Init connects and creates the versioned index if missing, returning true if it already existed.
func (i *Indexer) Init(ctx context.Context) (bool, error) {
	parsed, err := url.Parse(i.base)
	if err != nil {
		return false, fmt.Errorf("parse elasticsearch url: %w", err)
	}
	if parsed.User != nil {
		i.user = parsed.User.Username()
		i.pass, _ = parsed.User.Password()
		parsed.User = nil
	}
	base := parsed.String()
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	i.base = base
	// No client-level Timeout: bulk/_delete_by_query can legitimately run for
	// minutes on large repos. Per-request deadlines come from the caller's ctx;
	// transport-level timeouts cover stalled connects/handshakes/headers so a
	// half-open server cannot wedge the indexer indefinitely.
	i.client = &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          100,
		},
	}

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
	var body struct {
		Status string `json:"status"`
	}
	if err := i.doJSON(ctx, http.MethodGet, "_cluster/health", nil, &body); err != nil {
		return err
	}
	// Healthy = green; usable = yellow. Red is unusable.
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html
	if body.Status != "green" && body.Status != "yellow" {
		return fmt.Errorf("status of elasticsearch cluster is %s", body.Status)
	}
	return nil
}

// Close releases idle HTTP connections held by the client.
func (i *Indexer) Close() {
	if i == nil || i.client == nil {
		return
	}
	i.client.CloseIdleConnections()
	i.client = nil
}

// Bulk submits index/delete ops. Returns the first item-level failure, if any.
func (i *Indexer) Bulk(ctx context.Context, ops []BulkOp) error {
	if len(ops) == 0 {
		return nil
	}

	index := i.VersionedIndexName()
	var buf bytes.Buffer
	buf.Grow(len(ops) * 256)
	for _, op := range ops {
		meta := map[string]any{op.action: map[string]any{"_index": index, "_id": op.id}}
		if err := writeJSONLine(&buf, meta); err != nil {
			return err
		}
		if op.action == bulkActionIndex {
			if err := writeJSONLine(&buf, op.doc); err != nil {
				return err
			}
		}
	}

	res, err := i.do(ctx, http.MethodPost, urlPath(index, "_bulk"), "application/x-ndjson", bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	defer drainAndClose(res)

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
	if !body.Errors {
		return nil
	}
	return firstBulkError(body.Items)
}

// firstBulkError returns the first item-level failure in a bulk response.
// Each items entry is a single-key map ({"index": {...}} or {"delete": {...}}).
// Delete-of-missing (404) is idempotent and not reported.
func firstBulkError(items []map[string]struct {
	Status int        `json:"status"`
	Error  json.Value `json:"error"`
},
) error {
	for _, item := range items {
		for action, result := range item {
			if action == bulkActionDelete && result.Status == http.StatusNotFound {
				continue
			}
			if result.Status >= 300 {
				return fmt.Errorf("bulk %s failed (status %d): %s", action, result.Status, string(result.Error))
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
	return i.doJSON(ctx, http.MethodPut, urlPath(i.VersionedIndexName(), "_doc", id), bytes.NewReader(body), nil)
}

// Delete removes a single document by id. Missing ids are not an error.
func (i *Indexer) Delete(ctx context.Context, id string) error {
	res, err := i.do(ctx, http.MethodDelete, urlPath(i.VersionedIndexName(), "_doc", id), "", nil, http.StatusNotFound)
	if err != nil {
		return err
	}
	drainAndClose(res)
	return nil
}

// DeleteByQuery removes every document matching the query.
func (i *Indexer) DeleteByQuery(ctx context.Context, query Query) error {
	body, err := json.Marshal(map[string]any{"query": query.querySource()})
	if err != nil {
		return err
	}
	return i.doJSON(ctx, http.MethodPost, urlPath(i.VersionedIndexName(), "_delete_by_query"), bytes.NewReader(body), nil)
}

// Refresh forces a refresh so recent writes are searchable.
func (i *Indexer) Refresh(ctx context.Context) error {
	return i.doJSON(ctx, http.MethodPost, urlPath(i.VersionedIndexName(), "_refresh"), nil, nil)
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

	// Default track_total_hits is 10000 (capped count); send it explicitly so
	// callers can choose between exact totals (true) and skipping counting (false).
	path := urlPath(i.VersionedIndexName(), "_search") + "?track_total_hits=" + strconv.FormatBool(req.TrackTotal)
	res, err := i.do(ctx, http.MethodPost, path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer drainAndClose(res)
	return decodeSearchResponse(res.Body)
}

func (i *Indexer) indexExists(ctx context.Context, name string) (bool, error) {
	res, err := i.do(ctx, http.MethodHead, urlPath(name), "", nil, http.StatusNotFound)
	if err != nil {
		return false, err
	}
	drainAndClose(res)
	return res.StatusCode == http.StatusOK, nil
}

func (i *Indexer) createIndex(ctx context.Context) error {
	var body struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if err := i.doJSON(ctx, http.MethodPut, urlPath(i.VersionedIndexName()), bytes.NewBufferString(i.mapping), &body); err != nil {
		return fmt.Errorf("create index %s: %w", i.VersionedIndexName(), err)
	}
	if !body.Acknowledged {
		return fmt.Errorf("create index %s not acknowledged", i.VersionedIndexName())
	}

	i.checkOldIndexes(ctx)
	return nil
}

// do sends a request and returns the response. Status >= 300 is turned into
// an error unless the status appears in okStatus. The caller closes Body.
func (i *Indexer) do(ctx context.Context, method, path, contentType string, body io.Reader, okStatus ...int) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, i.base+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if i.user != "" || i.pass != "" {
		req.SetBasicAuth(i.user, i.pass)
	}
	res, err := i.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 && !slices.Contains(okStatus, res.StatusCode) {
		msg := readErrBody(res)
		res.Body.Close()
		return nil, fmt.Errorf("%s %s: %s", method, path, msg)
	}
	return res, nil
}

// doJSON sends a request with a JSON body and, when out is non-nil, decodes
// the JSON response into it.
func (i *Indexer) doJSON(ctx context.Context, method, path string, body io.Reader, out any) error {
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	res, err := i.do(ctx, method, path, contentType, body)
	if err != nil {
		return err
	}
	defer drainAndClose(res)
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// drainAndClose discards any unread response body before closing so the
// underlying TCP connection can be reused for keep-alive.
func drainAndClose(res *http.Response) {
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()
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

// readErrBody reads up to 4 KiB of an error response and drains the rest so
// the underlying connection can be reused (keep-alive needs Body fully read).
func readErrBody(res *http.Response) string {
	const limit = 4 << 10
	b, _ := io.ReadAll(io.LimitReader(res.Body, limit))
	_, _ = io.Copy(io.Discard, res.Body)
	return fmt.Sprintf("status %d: %s", res.StatusCode, bytes.TrimSpace(b))
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

// urlPath joins path segments with `/` and percent-escapes each.
func urlPath(segments ...string) string {
	var b bytes.Buffer
	for idx, s := range segments {
		if idx > 0 {
			b.WriteByte('/')
		}
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}
