// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"strings"
	"testing"
)

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
