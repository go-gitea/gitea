// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestGiteaUploadRepo(t *testing.T) {
	// FIXME: Since no accesskey or user/password will trigger rate limit of github, just skip
	t.Skip()

	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	var (
		downloader = NewGithubDownloaderV3(context.Background(), "https://github.com", "", "", "", "go-xorm", "builder")
		repoName   = "builder-" + time.Now().Format("2006-01-02-15-04-05")
		uploader   = NewGiteaLocalUploader(graceful.GetManager().HammerContext(), user, user.Name, repoName)
	)

	err := migrateRepository(downloader, uploader, base.MigrateOptions{
		CloneAddr:    "https://github.com/go-xorm/builder",
		RepoName:     repoName,
		AuthUsername: "",

		Wiki:         true,
		Issues:       true,
		Milestones:   true,
		Labels:       true,
		Releases:     true,
		Comments:     true,
		PullRequests: true,
		Private:      true,
		Mirror:       false,
	}, nil)
	assert.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName}).(*repo_model.Repository)
	assert.True(t, repo.HasWiki())
	assert.EqualValues(t, repo_model.RepositoryReady, repo.Status)

	milestones, _, err := models.GetMilestones(models.GetMilestonesOption{
		RepoID: repo.ID,
		State:  structs.StateOpen,
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 1)

	milestones, _, err = models.GetMilestones(models.GetMilestonesOption{
		RepoID: repo.ID,
		State:  structs.StateClosed,
	})
	assert.NoError(t, err)
	assert.Empty(t, milestones)

	labels, err := models.GetLabelsByRepoID(repo.ID, "", db.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, labels, 12)

	releases, err := models.GetReleasesByRepoID(repo.ID, models.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: true,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 8)

	releases, err = models.GetReleasesByRepoID(repo.ID, models.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: false,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	issues, err := models.Issues(&models.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsPull:   util.OptionalBoolFalse,
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 15)
	assert.NoError(t, issues[0].LoadDiscussComments())
	assert.Empty(t, issues[0].Comments)

	pulls, _, err := models.PullRequests(repo.ID, &models.PullRequestsOptions{
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, pulls, 30)
	assert.NoError(t, pulls[0].LoadIssue())
	assert.NoError(t, pulls[0].Issue.LoadDiscussComments())
	assert.Len(t, pulls[0].Issue.Comments, 2)
}

func TestGiteaUploadRemapLocalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	repoName := "migrated"
	uploader := NewGiteaLocalUploader(context.Background(), doer, doer.Name, repoName)
	// call remapLocalUser
	uploader.sameApp = true

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// The externalID does not match any existing user, everything
	// belongs to the doer
	//
	target := models.Release{}
	uploader.userMap = make(map[int64]int64)
	err := uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID matches a known user but the name does not match,
	// everything belongs to the doer
	//
	source.PublisherID = user.ID
	target = models.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// The externalID and externalName match an existing user, everything
	// belongs to the existing user
	//
	source.PublisherName = user.Name
	target = models.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, user.ID, target.GetUserID())
}

func TestGiteaUploadRemapExternalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	repoName := "migrated"
	uploader := NewGiteaLocalUploader(context.Background(), doer, doer.Name, repoName)
	uploader.gitServiceType = structs.GiteaService
	// call remapExternalUser
	uploader.sameApp = false

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// When there is no user linked to the external ID, the migrated data is authored
	// by the doer
	//
	uploader.userMap = make(map[int64]int64)
	target := models.Release{}
	err := uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, doer.ID, target.GetUserID())

	//
	// Link the external ID to an existing user
	//
	linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    strconv.FormatInt(externalID, 10),
		UserID:        linkedUser.ID,
		LoginSourceID: 0,
		Provider:      structs.GiteaService.Name(),
	}
	err = user_model.LinkExternalToUser(linkedUser, externalLoginUser)
	assert.NoError(t, err)

	//
	// When a user is linked to the external ID, it becomes the author of
	// the migrated data
	//
	uploader.userMap = make(map[int64]int64)
	target = models.Release{}
	err = uploader.remapUser(&source, &target)
	assert.NoError(t, err)
	assert.EqualValues(t, linkedUser.ID, target.GetUserID())
}
