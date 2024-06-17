// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripSlashesMiddleware(t *testing.T) {
	type test struct {
		name         string
		expectedPath string
		inputPath    string
	}

	tests := []test{
		{
			name:         "path with multiple slashes",
			inputPath:    "https://github.com///go-gitea//gitea.git",
			expectedPath: "/go-gitea/gitea.git",
		},
		{
			name:         "path with no slashes",
			inputPath:    "https://github.com/go-gitea/gitea.git",
			expectedPath: "/go-gitea/gitea.git",
		},
		{
			name:         "path with slashes in the middle",
			inputPath:    "https://git.data.coop//halfd/new-website.git",
			expectedPath: "/halfd/new-website.git",
		},
		{
			name:         "path with slashes in the middle",
			inputPath:    "https://git.data.coop//halfd/new-website.git",
			expectedPath: "/halfd/new-website.git",
		},
		{
			name:         "path with slashes in the end",
			inputPath:    "/user2//repo1/",
			expectedPath: "/user2/repo1",
		},
		{
			name:         "path with slashes and query params",
			inputPath:    "/repo//migrate?service_type=3",
			expectedPath: "/repo/migrate",
		},
		{
			name:         "path with encoded slash",
			inputPath:    "/user2/%2F%2Frepo1",
			expectedPath: "/user2/%2F%2Frepo1",
		},
	}

	for _, tt := range tests {
		testMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, tt.expectedPath, r.URL.Path)
		})

		// pass the test middleware to validate the changes
		handlerToTest := normalizeRequestPathMiddleware(testMiddleware)
		// create a mock request to use
		req := httptest.NewRequest("GET", tt.inputPath, nil)
		// call the handler using a mock response recorder
		handlerToTest.ServeHTTP(httptest.NewRecorder(), req)
	}
}
