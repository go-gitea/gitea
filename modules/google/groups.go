// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package google

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"

	"github.com/markbates/goth"
)

const (
	IAMScope                 = "https://www.googleapis.com/auth/cloud-identity.groups.readonly"
	defaultIAMGroupsEndpoint = "https://content-cloudidentity.googleapis.com/v1/groups/-/memberships:searchDirectGroups"
)

// maxGroupPages is the maximum number of pages fetched from the Google
// Cloud Identity API. The API returns up to 200 groups per page by default,
// so this caps group membership at 4,000 groups per user — far beyond any
// realistic Google Workspace organization.
const maxGroupPages = 20

// Client calls Google Workspace APIs.
type Client struct {
	httpClient                     *http.Client
	groupsEndpoint                 string
	claimName                      string
	failLoginOnAdditionalInfoError bool
}

// NewClient creates a Client using the given authenticated HTTP client.
// The client should be built from an OAuth2 token carrying IAMScope.
func NewClient(httpClient *http.Client, claimName string, failLoginOnAdditionalInfoError bool) *Client {
	return &Client{
		httpClient:                     httpClient,
		groupsEndpoint:                 defaultIAMGroupsEndpoint,
		claimName:                      claimName,
		failLoginOnAdditionalInfoError: failLoginOnAdditionalInfoError,
	}
}

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
func (c *Client) FetchGroups(ctx context.Context, email string) ([]string, error) {
	groups := make([]string, 0, 16)
	pageToken := ""

	for range maxGroupPages {
		params := url.Values{}
		params.Set("query", fmt.Sprintf("member_key_id=='%s'", email))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		apiURL := c.groupsEndpoint + "?" + params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("google groups: build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
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

// FetchAdditionalInfo implements oauth2.AdditionalInfoProvider.
// It fetches Google Workspace group memberships and injects them into
// gothUser.RawData under the given claimName key.
func (c *Client) FetchAdditionalInfo(ctx context.Context, user goth.User) (goth.User, error) {
	groups, err := c.FetchGroups(ctx, user.Email)
	if err != nil {
		return user, err
	}
	if user.RawData == nil {
		user.RawData = make(map[string]any)
	}
	if existing, has := user.RawData[c.claimName]; has {
		log.Warn("OAuth2 Google: RawData already contains claim %q with some value. Consider to use different claim name for groups information", c.claimName, existing)
	}
	user.RawData[c.claimName] = groups
	return user, nil
}

// FailLoginOnAdditionalInfoError implements oauth2.AdditionalInfoProvider.
func (c *Client) FailLoginOnAdditionalInfoError() bool {
	return c.failLoginOnAdditionalInfoError
}
