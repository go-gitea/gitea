// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/indexer/issues/internal/tests"

	"github.com/stretchr/testify/require"
)

func TestElasticsearchIndexer(t *testing.T) {
	// The elasticsearch instance started by pull-db-tests.yml > test-unit > services > elasticsearch
	rawURL := "http://elastic:changeme@elasticsearch:9200"

	if os.Getenv("CI") == "" {
		// Make it possible to run tests against a local elasticsearch instance
		rawURL = os.Getenv("TEST_ELASTICSEARCH_URL")
		if rawURL == "" {
			t.Skip("TEST_ELASTICSEARCH_URL not set and not running in CI")
			return
		}
	}

	// Go's net/http does not auto-attach URL userinfo as Basic Auth, so extract
	// it and set the header explicitly; otherwise auth-enforced clusters answer
	// 401 and the probe never reports ready.
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	user := parsed.User
	parsed.User = nil
	probeURL := parsed.String()

	require.Eventually(t, func() bool {
		req, err := http.NewRequest(http.MethodGet, probeURL, nil)
		if err != nil {
			return false
		}
		if user != nil {
			pass, _ := user.Password()
			req.SetBasicAuth(user.Username(), pass)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, time.Minute, time.Second, "Expected elasticsearch to be up")

	indexer := NewIndexer(rawURL, fmt.Sprintf("test_elasticsearch_indexer_%d", time.Now().Unix()))
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}
