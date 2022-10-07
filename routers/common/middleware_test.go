// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
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
			expectedPath: "https://github.com/go-gitea/gitea.git",
		},
		{
			name:         "path with no slashes",
			inputPath:    "https://github.com/go-gitea/gitea.git",
			expectedPath: "https://github.com/go-gitea/gitea.git",
		},
		{
			name:         "path with slashes in the middle",
			inputPath:    "https://git.data.coop//halfd/new-website.git",
			expectedPath: "https://git.data.coop/halfd/new-website.git",
		},
		{
			name:         "path with slashes in the middle",
			inputPath:    "https://git.data.coop//halfd/new-website.git",
			expectedPath: "https://git.data.coop/halfd/new-website.git",
		},
		{
			name:         "path with slashes in the end",
			inputPath:    "/user2//repo1/",
			expectedPath: "/user2/repo1",
		},
		{
			name:         "path with slashes and query params",
			inputPath:    "/repo//migrate?service_type=3",
			expectedPath: "/repo/migrate?service_type=3",
		},
	}

	for _, tt := range tests {
		testMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, tt.expectedPath, r.URL.String())
		})

		// pass the test middleware to validate the changes
		handlerToTest := stripSlashesMiddleware(testMiddleware)
		// create a mock request to use
		req := httptest.NewRequest("GET", tt.inputPath, nil)
		// call the handler using a mock response recorder
		handlerToTest.ServeHTTP(httptest.NewRecorder(), req)
	}
}
