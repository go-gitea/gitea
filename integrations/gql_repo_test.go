// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGQLViewRepo(t *testing.T) {
	defer prepareTestEnv(t)()
	body := `{
			"query":
				"query GetRepo {
					repository(owner:"user2", name:"repo1") {
						id,
						rest_api_id,
						name,
						full_name,
						release_counter,
						size,
						description,
						empty,
						private,
						fork,
						template,
						owner {
							username,
							id,
							rest_api_id
						},
						allow_merge_commits,
						open_issues_count,
						open_pr_counter
					}
				}",
				"operationName":"GetRepo"
		}`

	req := NewRequestWithBody(t, "GET", "/api/graphql", strings.NewReader(body))
	resp := MakeRequest(t, req, http.StatusOK)
	jsonMap := make(map[string](interface{}))
	DecodeJSON(t, resp, &jsonMap)

	dataMap := jsonMap["data"].(map[string]interface{})
	repositoryMap := dataMap["repository"].(map[string]interface{})
	assert.EqualValues(t, 1, repositoryMap["rest_api_id"])
	assert.EqualValues(t, "repo1", repositoryMap["name"])
	assert.EqualValues(t, 1, repositoryMap["release_counter"])
	assert.EqualValues(t, 1, repositoryMap["open_issues_count"])
	assert.EqualValues(t, 3, repositoryMap["open_pr_counter"])
}
