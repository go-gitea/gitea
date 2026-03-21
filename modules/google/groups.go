// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package google

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/json"
)

const IAMScope = "https://www.googleapis.com/auth/cloud-identity.groups.readonly"

var IAMGroupsEndpoint = "https://content-cloudidentity.googleapis.com/v1/groups/-/memberships:searchDirectGroups"

// groupMembership represents a single membership entry returned by the
// Cloud Identity Groups API searchDirectGroups endpoint.
type groupMembership struct {
	GroupKey struct {
		ID string `json:"id"`
	} `json:"groupKey"`
}

// groupsResponse is the paged response from the Cloud Identity API.
type groupsResponse struct {
	Memberships   []groupMembership `json:"memberships"`
	NextPageToken string            `json:"nextPageToken"`
}

// FetchGroups queries the Google Cloud Identity Groups API for all
// groups the given user (identified by email) is a direct member of.
// The caller must supply an HTTP client already authenticated with an access
// token that carries the IAMScope scope.
func FetchGroups(ctx context.Context, client *http.Client, email string) ([]string, error) {
	groups := make([]string, 0, 16)
	pageToken := ""

	for {
		url := fmt.Sprintf("%s?query=member_key_id=='%s'", IAMGroupsEndpoint, email)
		if pageToken != "" {
			url = fmt.Sprintf("%s&pageToken=%s", url, pageToken)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("google groups: build request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("google groups: HTTP request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("google groups: read response: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("google groups: API returned %d: %s", resp.StatusCode, body)
		}

		var page groupsResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("google groups: decode response: %w", err)
		}

		for _, m := range page.Memberships {
			if m.GroupKey.ID != "" {
				groups = append(groups, m.GroupKey.ID)
			}
		}

		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	return groups, nil
}
