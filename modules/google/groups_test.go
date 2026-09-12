// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package google

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c := NewClient(server.Client(), "groups", false)
	c.groupsEndpoint = server.URL
	return c
}

func mockGroupsServer(t *testing.T, expectedEmail string, pages [][]string) *httptest.Server {
	t.Helper()
	pageIndex := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the query contains the correct member_key_id
		expectedQuery := fmt.Sprintf("member_key_id=='%s'", expectedEmail)
		assert.Equal(t, expectedQuery, r.URL.Query().Get("query"))

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
	server := mockGroupsServer(t, "user@example.com", [][]string{
		{"group-a@example.com", "group-b@example.com"},
	})
	defer server.Close()

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"group-a@example.com", "group-b@example.com"}, groups)
}

func TestFetchGroups_SinglePage(t *testing.T) {
	server := mockGroupsServer(t, "user@example.com", [][]string{
		{"group-a@example.com", "group-b@example.com"},
	})
	defer server.Close()

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"group-a@example.com", "group-b@example.com"}, groups)
}

func TestFetchGoogleGroups_MultiPage(t *testing.T) {
	server := mockGroupsServer(t, "user@example.com", [][]string{
		{"group-a@example.com"},
		{"group-b@example.com", "group-c@example.com"},
	})
	defer server.Close()

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"group-a@example.com", "group-b@example.com", "group-c@example.com"}, groups)
}

func TestFetchGoogleGroups_Empty(t *testing.T) {
	server := mockGroupsServer(t, "user@example.com", [][]string{{}})
	defer server.Close()

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestFetchGoogleGroups_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
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

	client := newTestClient(t, server)
	groups, err := client.FetchGroups(context.Background(), "user@example.com")
	require.Error(t, err)
	assert.Nil(t, groups)
}

func TestFetchAdditionalInfo_InjectsClaimBeforeValidation(t *testing.T) {
	server := mockGroupsServer(t, "user@example.com", [][]string{
		{"required-group@example.com"},
	})
	defer server.Close()

	c := newTestClient(t, server)
	c.claimName = "groups"

	user := goth.User{
		Email:   "user@example.com",
		RawData: map[string]any{},
	}

	enriched, err := c.FetchAdditionalInfo(context.Background(), user)
	require.NoError(t, err)

	// Verify the claim is present and contains the group — simulating what
	// RequiredClaimName validation would check after enrichment runs.
	groups, ok := enriched.RawData["groups"]
	require.True(t, ok, "groups claim must be present in RawData after enrichment")
	groupSlice, ok := groups.([]string)
	require.True(t, ok)
	assert.Contains(t, groupSlice, "required-group@example.com")
}

func TestFetchAdditionalInfo_ErrorDoesNotInjectClaim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	c.claimName = "groups"

	user := goth.User{
		Email:   "user@example.com",
		RawData: map[string]any{},
	}

	enriched, err := c.FetchAdditionalInfo(context.Background(), user)
	require.Error(t, err)
	_, hasGroups := enriched.RawData["groups"]
	assert.False(t, hasGroups)
}
