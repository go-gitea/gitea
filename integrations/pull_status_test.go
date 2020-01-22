// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestPullCreate_CommitStatus(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetCSRF(t, session, url),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusFound)

		req = NewRequest(t, "GET", "/user1/repo1/pulls")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		// Request repository commits page
		req = NewRequest(t, "GET", "/user1/repo1/pulls/1/commits")
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)

		// Get first commit URL
		commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)

		commitID := path.Base(commitURL)

		statusList := []api.CommitStatusState{
			api.CommitStatusPending,
			api.CommitStatusError,
			api.CommitStatusFailure,
			api.CommitStatusWarning,
			api.CommitStatusSuccess,
		}

		statesIcons := map[api.CommitStatusState]string{
			api.CommitStatusPending: "circle icon yellow",
			api.CommitStatusSuccess: "check icon green",
			api.CommitStatusError:   "warning icon red",
			api.CommitStatusFailure: "remove icon red",
			api.CommitStatusWarning: "warning sign icon yellow",
		}

		// Update commit status, and check if icon is updated as well
		for _, status := range statusList {

			// Call API to add status for commit
			token := getTokenForLoggedInUser(t, session)
			req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/user1/repo1/statuses/%s?token=%s", commitID, token),
				api.CreateStatusOption{
					State:       api.StatusState(status),
					TargetURL:   "http://test.ci/",
					Description: "",
					Context:     "testci",
				},
			)
			session.MakeRequest(t, req, http.StatusCreated)

			req = NewRequestf(t, "GET", "/user1/repo1/pulls/1/commits")
			resp = session.MakeRequest(t, req, http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)

			commitURL, exists = doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
			assert.True(t, exists)
			assert.NotEmpty(t, commitURL)
			assert.EqualValues(t, commitID, path.Base(commitURL))

			cls, ok := doc.doc.Find("#commits-table tbody tr td.message i.commit-status").Last().Attr("class")
			assert.True(t, ok)
			assert.EqualValues(t, "commit-status "+statesIcons[status], cls)
		}
	})
}

func TestPullCreate_EmptyChangesWithCommits(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "status1", "status1", "README.md", "# repo1\n\nDescription for repo1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetCSRF(t, session, url),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusFound)

		req = NewRequest(t, "GET", "/user1/repo1/pulls/1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		text := strings.TrimSpace(doc.doc.Find(".item.text.green").Text())
		assert.EqualValues(t, "This pull request can be merged automatically.", text)
	})
}
