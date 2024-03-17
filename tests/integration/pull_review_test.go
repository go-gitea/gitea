// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	repo_service "code.gitea.io/gitea/services/repository"
	files_service "code.gitea.io/gitea/services/repository/files"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPullView_ReviewerMissed(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	req := NewRequest(t, "GET", "/pulls")
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user2/repo1/pulls/3")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestPullView_CodeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// Create the repo.
		repo, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user2, user2, repo_service.CreateRepoOptions{
			Name:             "test_codeowner",
			Readme:           "Default",
			AutoInit:         true,
			ObjectFormatName: git.Sha1ObjectFormat.Name(),
			DefaultBranch:    "master",
		})
		assert.NoError(t, err)

		// add CODEOWNERS to default branch
		_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			OldBranch: repo.DefaultBranch,
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("README.md @user5\n"),
				},
			},
		})
		assert.NoError(t, err)

		t.Run("First Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new project\n"),
					},
				},
			})
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch", "Test Pull Request")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "codeowner-basebranch"})
			unittest.AssertExistsIf(t, true, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 5})
		})

		// change the default branch CODEOWNERS file to change README.md's codeowner
		_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{
				{
					Operation:     "update",
					TreePath:      "CODEOWNERS",
					ContentReader: strings.NewReader("README.md @user8\n"),
				},
			},
		})
		assert.NoError(t, err)

		t.Run("Second Pull Request", func(t *testing.T) {
			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, repo, user2, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch2",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new project2\n"),
					},
				},
			})
			assert.NoError(t, err)

			// Create a pull request.
			session := loginUser(t, "user2")
			testPullCreate(t, session, "user2", "test_codeowner", false, repo.DefaultBranch, "codeowner-basebranch2", "Test Pull Request2")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadBranch: "codeowner-basebranch2"})
			unittest.AssertExistsIf(t, true, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
		})

		t.Run("Forked Repo Pull Request", func(t *testing.T) {
			user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
			forkedRepo, err := repo_service.ForkRepository(db.DefaultContext, user2, user5, repo_service.ForkRepoOptions{
				BaseRepo: repo,
				Name:     "test_codeowner",
			})
			assert.NoError(t, err)

			// create a new branch to prepare for pull request
			_, err = files_service.ChangeRepoFiles(db.DefaultContext, forkedRepo, user5, &files_service.ChangeRepoFilesOptions{
				NewBranch: "codeowner-basebranch-forked",
				Files: []*files_service.ChangeRepoFile{
					{
						Operation:     "update",
						TreePath:      "README.md",
						ContentReader: strings.NewReader("# This is a new forked project\n"),
					},
				},
			})
			assert.NoError(t, err)

			session := loginUser(t, "user5")
			testPullCreate(t, session, "user5", "test_codeowner", true, forkedRepo.DefaultBranch, "codeowner-basebranch-forked", "Test Pull Request2")

			pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{BaseRepoID: forkedRepo.ID, HeadBranch: "codeowner-basebranch-forked"})
			unittest.AssertExistsIf(t, false, &issues_model.Review{IssueID: pr.IssueID, Type: issues_model.ReviewTypeRequest, ReviewerID: 8})
		})
	})
}
