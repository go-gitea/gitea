// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestPullRevisions(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base", "README.md", "Hello, World (Edited Twice)\n")

		user1 := models.AssertExistsAndLoadBean(t, &models.User{
			Name: "user1",
		}).(*models.User)
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		}).(*models.Repository)

		testEditFileToNewBranch(t, session, "user1", "repo1", "base", "head", "README.md", "Hello, World (Edited Once)\n")

		// Use API to create a conflicting pr
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls?token=%s", "user1", "repo1", token), &api.CreatePullRequestOption{
			Head:  "head",
			Base:  "base",
			Title: "revisions",
		})
		session.MakeRequest(t, req, 201)

		pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "head",
			BaseBranch: "base",
		}).(*models.PullRequest)

		req = NewRequest(t, "GET", fmt.Sprintf("/user1/repo1/pulls/%d/revisions", pr.Index))
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		revisions := htmlDoc.doc.Find("td.revision")
		assert.Equal(t, 1, len(revisions.Nodes), "The template has changed")

		testEditFile(t, session, "user1", "repo1", "head", "README.md", "Revision2")

		//Wait for AddTestPullRequestTask
		time.Sleep(2 * time.Second)

		req = NewRequest(t, "GET", fmt.Sprintf("/user1/repo1/pulls/%d/revisions", pr.Index))
		resp = session.MakeRequest(t, req, http.StatusOK)

		htmlDoc = NewHTMLParser(t, resp.Body)
		revisions = htmlDoc.doc.Find("td.revision")
		assert.Equal(t, 2, len(revisions.Nodes), "The template has changed")
	})
}
