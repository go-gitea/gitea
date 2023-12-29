// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/test"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func assertBadge(t *testing.T, resp *httptest.ResponseRecorder, badge string) {
	assert.Equal(t, fmt.Sprintf("https://img.shields.io/badge/%s", badge), test.RedirectURL(resp))
}

func createMinimalRepo(t *testing.T) func() {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create a new repository
	repo, err := repo_service.CreateRepository(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
		Name:          "minimal",
		Description:   "minimal repo for badge testing",
		AutoInit:      true,
		Gitignores:    "Go",
		License:       "MIT",
		Readme:        "Default",
		DefaultBranch: "main",
		IsPrivate:     false,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, repo)

	// Enable Actions, and disable Issues, PRs and Releases
	err = repo_model.UpdateRepositoryUnits(db.DefaultContext, repo, []repo_model.RepoUnit{{
		RepoID: repo.ID,
		Type:   unit_model.TypeActions,
	}}, []unit_model.Type{unit_model.TypeIssues, unit_model.TypePullRequests, unit_model.TypeReleases})
	assert.NoError(t, err)

	return func() {
		repo_service.DeleteRepository(db.DefaultContext, user2, repo, false)
	}
}

func addWorkflow(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user2", "minimal")
	assert.NoError(t, err)

	// Add a workflow file to the repo
	addWorkflowToBaseResp, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     "create",
				TreePath:      ".gitea/workflows/pr.yml",
				ContentReader: strings.NewReader("name: test\non:\n  push:\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
			},
		},
		Message:   "add workflow",
		OldBranch: "main",
		NewBranch: "main",
		Author: &files_service.IdentityOptions{
			Name:  user2.Name,
			Email: user2.Email,
		},
		Committer: &files_service.IdentityOptions{
			Name:  user2.Name,
			Email: user2.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    time.Now(),
			Committer: time.Now(),
		},
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, addWorkflowToBaseResp)

	assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))
}

func TestWorkflowBadges(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer tests.PrintCurrentTest(t)()
		defer createMinimalRepo(t)()

		addWorkflow(t)

		// Actions disabled
		req := NewRequest(t, "GET", "/user2/repo1/badges/workflows/test.yaml/badge.svg")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "test.yaml-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/repo1/badges/workflows/test.yaml/badge.svg?branch=no-such-branch")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "test.yaml-Not%20found-crimson")

		// Actions enabled
		req = NewRequest(t, "GET", "/user2/minimal/badges/workflows/pr.yml/badge.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-waiting-lightgrey")

		req = NewRequest(t, "GET", "/user2/minimal/badges/workflows/pr.yml/badge.svg?branch=main")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-waiting-lightgrey")

		req = NewRequest(t, "GET", "/user2/minimal/badges/workflows/pr.yml/badge.svg?branch=no-such-branch")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/badges/workflows/pr.yml/badge.svg?event=cron")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-Not%20found-crimson")

		// GitHub compatibility
		req = NewRequest(t, "GET", "/user2/minimal/actions/workflows/pr.yml/badge.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-waiting-lightgrey")

		req = NewRequest(t, "GET", "/user2/minimal/actions/workflows/pr.yml/badge.svg?branch=main")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-waiting-lightgrey")

		req = NewRequest(t, "GET", "/user2/minimal/actions/workflows/pr.yml/badge.svg?branch=no-such-branch")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/actions/workflows/pr.yml/badge.svg?event=cron")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pr.yml-Not%20found-crimson")
	})
}

func TestBadges(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Stars", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/badges/stars.svg")
		resp := MakeRequest(t, req, http.StatusSeeOther)

		assertBadge(t, resp, "stars-0-blue")
	})

	t.Run("Issues", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer createMinimalRepo(t)()

		// Issues enabled
		req := NewRequest(t, "GET", "/user2/repo1/badges/issues.svg")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-2-blue")

		req = NewRequest(t, "GET", "/user2/repo1/badges/issues/open.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-1%20open-blue")

		req = NewRequest(t, "GET", "/user2/repo1/badges/issues/closed.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-1%20closed-blue")

		// Issues disabled
		req = NewRequest(t, "GET", "/user2/minimal/badges/issues.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/badges/issues/open.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/badges/issues/closed.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "issues-Not%20found-crimson")
	})

	t.Run("Pulls", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer createMinimalRepo(t)()

		// Pull requests enabled
		req := NewRequest(t, "GET", "/user2/repo1/badges/pulls.svg")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-3-blue")

		req = NewRequest(t, "GET", "/user2/repo1/badges/pulls/open.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-3%20open-blue")

		req = NewRequest(t, "GET", "/user2/repo1/badges/pulls/closed.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-0%20closed-blue")

		// Pull requests disabled
		req = NewRequest(t, "GET", "/user2/minimal/badges/pulls.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/badges/pulls/open.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-Not%20found-crimson")

		req = NewRequest(t, "GET", "/user2/minimal/badges/pulls/closed.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "pulls-Not%20found-crimson")
	})

	t.Run("Release", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer createMinimalRepo(t)()

		req := NewRequest(t, "GET", "/user2/repo1/badges/release.svg")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "release-v1.1-blue")

		req = NewRequest(t, "GET", "/user2/minimal/badges/release.svg")
		resp = MakeRequest(t, req, http.StatusSeeOther)
		assertBadge(t, resp, "release-Not%20found-crimson")
	})
}
