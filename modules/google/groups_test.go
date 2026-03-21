// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package google

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockGroupsServer(t *testing.T, pages [][]string) *httptest.Server {
	t.Helper()
	pageIndex := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var memberships []string
		for _, g := range pages[pageIndex] {
			memberships = append(memberships, fmt.Sprintf(`{"groupKey":{"id":%q}}`, g))
		}

		nextPageToken := ""
		if pageIndex < len(pages)-1 {
			nextPageToken = "page-token"
		}
		pageIndex++

		body := fmt.Sprintf(
			`{"memberships":[%s],"nextPageToken":%q}`,
			strings.Join(memberships, ","),
			nextPageToken,
		)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
}

func TestFetchGoogleGroups_SinglePage(t *testing.T) {
	server := mockGroupsServer(t, [][]string{
		{"group-a@example.com", "group-b@example.com"},
	})
	defer server.Close()

	origEndpoint := IAMGroupsEndpoint
	IAMGroupsEndpoint = server.URL
	defer func() { IAMGroupsEndpoint = origEndpoint }()

	groups, err := FetchGroups(context.Background(), &http.Client{}, "user@example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"group-a@example.com", "group-b@example.com"}, groups)
}

func TestFetchGoogleGroups_MultiPage(t *testing.T) {
	server := mockGroupsServer(t, [][]string{
		{"group-a@example.com"},
		{"group-b@example.com", "group-c@example.com"},
	})
	defer server.Close()

	origEndpoint := IAMGroupsEndpoint
	IAMGroupsEndpoint = server.URL
	defer func() { IAMGroupsEndpoint = origEndpoint }()

	groups, err := FetchGroups(context.Background(), &http.Client{}, "user@example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"group-a@example.com", "group-b@example.com", "group-c@example.com"}, groups)
}

func TestFetchGoogleGroups_Empty(t *testing.T) {
	server := mockGroupsServer(t, [][]string{{}})
	defer server.Close()

	origEndpoint := IAMGroupsEndpoint
	IAMGroupsEndpoint = server.URL
	defer func() { IAMGroupsEndpoint = origEndpoint }()

	groups, err := FetchGroups(context.Background(), &http.Client{}, "user@example.com")
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestFetchGoogleGroups_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer server.Close()

	origEndpoint := IAMGroupsEndpoint
	IAMGroupsEndpoint = server.URL
	defer func() { IAMGroupsEndpoint = origEndpoint }()

	groups, err := FetchGroups(context.Background(), &http.Client{}, "user@example.com")
	require.Error(t, err)
	assert.Nil(t, groups)
	assert.Contains(t, err.Error(), "403")
}

func TestFetchGoogleGroups_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not valid json`)
	}))
	defer server.Close()

	origEndpoint := IAMGroupsEndpoint
	IAMGroupsEndpoint = server.URL
	defer func() { IAMGroupsEndpoint = origEndpoint }()

	groups, err := FetchGroups(context.Background(), &http.Client{}, "user@example.com")
	require.Error(t, err)
	assert.Nil(t, groups)
}
