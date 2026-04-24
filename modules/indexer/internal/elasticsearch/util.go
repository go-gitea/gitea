// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"

	"github.com/elastic/go-elasticsearch/v8"
)

// VersionedIndexName returns the full index name with version suffix.
func (i *Indexer) VersionedIndexName() string {
	return versionedIndexName(i.indexName, i.version)
}

func versionedIndexName(indexName string, version int) string {
	if version == 0 {
		// Old index name without version
		return indexName
	}
	return fmt.Sprintf("%s.v%d", indexName, version)
}

// newClient builds the underlying go-elasticsearch client.
// The configured URL may embed credentials (http://user:pass@host); split
// them out because the v8 client prefers explicit Username/Password.
func (i *Indexer) newClient() (*elasticsearch.Client, error) {
	parsed, err := url.Parse(i.url)
	if err != nil {
		return nil, fmt.Errorf("parse elasticsearch url: %w", err)
	}
	cfg := elasticsearch.Config{}
	if parsed.User != nil {
		cfg.Username = parsed.User.Username()
		cfg.Password, _ = parsed.User.Password()
		parsed.User = nil
	}
	cfg.Addresses = []string{parsed.String()}
	// The v8 client refuses to talk to a server that does not advertise
	// `X-Elastic-Product: Elasticsearch`. Probe the root endpoint once to
	// detect OpenSearch (which deliberately omits the header) and only
	// install the shim in that case; real ES still benefits from the
	// client's built-in server-identity check.
	if isOpenSearch(i.url) {
		cfg.Transport = &productHeaderShim{base: http.DefaultTransport}
	}
	return elasticsearch.NewClient(cfg)
}

type productHeaderShim struct{ base http.RoundTripper }

func (p *productHeaderShim) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := p.base.RoundTrip(req)
	if resp != nil && resp.Header.Get("X-Elastic-Product") == "" {
		resp.Header.Set("X-Elastic-Product", "Elasticsearch")
	}
	return resp, err
}

// isOpenSearch reports whether the server at rawURL self-identifies as
// OpenSearch via its root endpoint. Any transport error or non-OpenSearch
// response is treated as "no": callers fall back to the default v8 client
// path so a real ES cluster keeps its product-identity check.
func isOpenSearch(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	// Go's net/http does not auto-attach URL userinfo as Basic Auth, so
	// extract it and set the header explicitly; otherwise authenticated
	// OpenSearch clusters answer 401 and the probe misidentifies them.
	user := parsed.User
	parsed.User = nil
	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return false
	}
	if user != nil {
		pass, _ := user.Password()
		req.SetBasicAuth(user.Username(), pass)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil || resp == nil {
		return false
	}
	defer resp.Body.Close()
	var body struct {
		Version struct {
			Distribution string `json:"distribution"`
		} `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	return body.Version.Distribution == "opensearch"
}

func (i *Indexer) checkOldIndexes(ctx context.Context) {
	for v := 0; v < i.version; v++ {
		indexName := versionedIndexName(i.indexName, v)
		exists, err := i.indexExists(ctx, indexName)
		if err == nil && exists {
			log.Warn("Found older elasticsearch index named %q, Gitea will keep the old NOT DELETED. You can delete the old version after the upgrade succeed.", indexName)
		}
	}
}
