// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoListSort(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadOrganization)

	for _, tc := range []struct {
		name, path, token string
	}{
		{"User", "/api/v1/users/user2/repos", token},
		{"CurrentUser", "/api/v1/user/repos", token},
		{"Organization", "/api/v1/orgs/org3/repos", token},
		{"AnonymousUser", "/api/v1/users/user2/repos", ""},
		{"AnonymousOrganization", "/api/v1/orgs/org3/repos", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list := func(query string) []int64 {
				t.Helper()
				resp := MakeRequest(t, NewRequest(t, "GET", tc.path+query).AddTokenAuth(tc.token), http.StatusOK)
				repos := DecodeJSON(t, resp, []*api.Repository{})
				ids := make([]int64, len(repos))
				for i, repo := range repos {
					ids[i] = repo.ID
					if tc.token == "" {
						assert.False(t, repo.Private)
					}
				}
				return ids
			}
			original := list("?limit=50")
			require.NotEmpty(t, original)
			for _, id := range original {
				require.NoError(t, repo_model.UpdateRepositoryUpdatedTime(t.Context(), id, time.Unix(1000+(id%3)*1000, 0)))
			}
			assert.Equal(t, original, list("?limit=50"))
			assert.Equal(t, original, list("?limit=50&order=desc"))

			expected := slices.Clone(original)
			slices.SortFunc(expected, func(a, b int64) int {
				if order := cmp.Compare(a%3, b%3); order != 0 {
					return order
				}
				return cmp.Compare(a, b)
			})
			assert.Equal(t, expected, list("?limit=50&sort=updated"))
			assert.Equal(t, expected, list("?limit=50&sort=updated&order=asc"))
			slices.Reverse(expected)
			assert.Equal(t, expected, list("?limit=50&sort=updated&order=desc"))

			var paged []int64
			for page := 1; len(paged) < len(expected); page++ {
				resp := MakeRequest(t, NewRequestf(t, "GET", "%s?sort=updated&order=desc&limit=2&page=%d", tc.path, page).AddTokenAuth(tc.token), http.StatusOK)
				assert.Equal(t, strconv.Itoa(len(expected)), resp.Header().Get("X-Total-Count"))
				repos := DecodeJSON(t, resp, []*api.Repository{})
				require.NotEmpty(t, repos)
				for _, repo := range repos {
					paged = append(paged, repo.ID)
				}
			}
			assert.Equal(t, expected, paged)

			for _, query := range []string{"sort=invalid", "sort=updated&order=invalid"} {
				resp := MakeRequest(t, NewRequest(t, "GET", tc.path+"?"+query).AddTokenAuth(tc.token), http.StatusUnprocessableEntity)
				assert.Contains(t, resp.Body.String(), "Invalid sort")
			}
			assert.Empty(t, list(fmt.Sprintf("?sort=updated&limit=2&page=%d", len(expected)+1)))
		})
	}
}
