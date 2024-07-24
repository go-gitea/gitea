// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"version":{"number":"8.12.1"}}`)
	}))
	defer server.Close()

	version, err := DetectVersion(server.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if version != 8 {
		t.Errorf("Expected version 8 but got %d", version)
	}

	// error
	version, err = DetectVersion("http://not-found:1234")
	if err == nil {
		t.Errorf("Expected error but got nil")
	}
	if version != 0 {
		t.Errorf("Expected version 0 but got %d", version)
	}
}

func TestParseElasticVersion(t *testing.T) {
	tests := []struct {
		content  string
		version  int
		hasError bool
	}{
		{
			content: `{"name":"instance-0000000000","cluster_name":"my_test_cluster","version":{"number":"7.0.0"}}`,
			version: 7,
		},
		{
			content: `{"version":{"number":"8.12.1"}}`,
			version: 8,
		},
		{
			content:  `{"version":{"number":"6.0.0"}}`,
			version:  0,
			hasError: true,
		},
		{
			content:  `{"version":{"number":"7-0-0"}}`,
			version:  0,
			hasError: true,
		},
		{
			content:  ``,
			version:  0,
			hasError: true,
		},
	}

	for _, test := range tests {
		version, err := parseElasticVersion(strings.NewReader(test.content))
		if test.hasError && err == nil {
			t.Errorf("Expected error but got nil")
		}
		if !test.hasError && err != nil {
			t.Errorf("Expected no error but got %v", err)
		}
		if version != test.version {
			t.Errorf("Expected version %d but got %d", test.version, version)
		}
	}
}
