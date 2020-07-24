// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repofiles"
	repo_module "code.gitea.io/gitea/modules/repository"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestPullUpdate(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		//Create PR to test
		user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
		org26 := models.AssertExistsAndLoadBean(t, &models.User{ID: 26}).(*models.User)
		pr := createOutdatedPR(t, user, org26)

		//Test GetDiverging
		diffCount, err := pull_service.GetDiverging(pr)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, diffCount.Behind)
		assert.EqualValues(t, 1, diffCount.Ahead)

		message := fmt.Sprintf("Merge branch '%s' into %s", pr.BaseBranch, pr.HeadBranch)
		err = pull_service.Update(pr, user, message)
		assert.NoError(t, err)

		//Test GetDiverging after update
		diffCount, err = pull_service.GetDiverging(pr)
		assert.NoError(t, err)
		assert.EqualValues(t, 0, diffCount.Behind)
		assert.EqualValues(t, 2, diffCount.Ahead)

	})
}

func createOutdatedPR(t *testing.T, actor, forkOrg *models.User) *models.PullRequest {
	baseRepo, err := repo_service.CreateRepository(actor, actor, models.CreateRepoOptions{
		Name:        "repo-pr-update",
		Description: "repo-tmp-pr-update description",
		AutoInit:    true,
		Gitignores:  "C,C++",
		License:     "MIT",
		Readme:      "Default",
		IsPrivate:   false,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, baseRepo)

	headRepo, err := repo_module.ForkRepository(actor, forkOrg, baseRepo, "repo-pr-update", "desc")
	assert.NoError(t, err)
	assert.NotEmpty(t, headRepo)

	//create a commit on base Repo
	_, err = repofiles.CreateOrUpdateRepoFile(baseRepo, actor, &repofiles.UpdateRepoFileOptions{
		TreePath:  "File_A",
		Message:   "Add File A",
		Content:   "File A",
		IsNewFile: true,
		OldBranch: "master",
		NewBranch: "master",
		Author: &repofiles.IdentityOptions{
			Name:  actor.Name,
			Email: actor.Email,
		},
		Committer: &repofiles.IdentityOptions{
			Name:  actor.Name,
			Email: actor.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    time.Now(),
			Committer: time.Now(),
		},
	})
	assert.NoError(t, err)

	//create a commit on head Repo
	_, err = repofiles.CreateOrUpdateRepoFile(headRepo, actor, &repofiles.UpdateRepoFileOptions{
		TreePath:  "File_B",
		Message:   "Add File on PR branch",
		Content:   "File B",
		IsNewFile: true,
		OldBranch: "master",
		NewBranch: "newBranch",
		Author: &repofiles.IdentityOptions{
			Name:  actor.Name,
			Email: actor.Email,
		},
		Committer: &repofiles.IdentityOptions{
			Name:  actor.Name,
			Email: actor.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    time.Now(),
			Committer: time.Now(),
		},
	})
	assert.NoError(t, err)

	//create Pull
	pullIssue := &models.Issue{
		RepoID:   baseRepo.ID,
		Title:    "Test Pull -to-update-",
		PosterID: actor.ID,
		Poster:   actor,
		IsPull:   true,
	}
	pullRequest := &models.PullRequest{
		HeadRepoID: headRepo.ID,
		BaseRepoID: baseRepo.ID,
		HeadBranch: "newBranch",
		BaseBranch: "master",
		HeadRepo:   headRepo,
		BaseRepo:   baseRepo,
		Type:       models.PullRequestGitea,
	}
	err = pull_service.NewPullRequest(baseRepo, pullIssue, nil, nil, pullRequest, nil)
	assert.NoError(t, err)

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{Title: "Test Pull -to-update-"}).(*models.Issue)
	pr, err := models.GetPullRequestByIssueID(issue.ID)
	assert.NoError(t, err)

	return pr
}
