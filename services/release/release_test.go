// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"strings"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/attachment"
	"gitea.dev/services/context/upload"

	_ "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRelease_Create(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	gitRepo, err := git.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1",
		Target:       "master",
		Title:        "v0.1 is released",
		Note:         "v0.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.1",
		Target:       "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Title:        "v0.1.1 is released",
		Note:         "v0.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.2",
		Target:       "65f1bf2",
		Title:        "v0.1.2 is released",
		Note:         "v0.1.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.3",
		Target:       "65f1bf2",
		Title:        "v0.1.3 is released",
		Note:         "v0.1.3 is released",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.4",
		Target:       "65f1bf2",
		Title:        "v0.1.4 is released",
		Note:         "v0.1.4 is released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil, ""))

	testPlayload := "testtest"

	attach, err := attachment.NewAttachment(t.Context(), &repo_model.Attachment{
		RepoID:     repo.ID,
		UploaderID: user.ID,
		Name:       "test.txt",
	}, strings.NewReader(testPlayload), int64(len([]byte(testPlayload))))
	assert.NoError(t, err)

	release := repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.5",
		Target:       "65f1bf2",
		Title:        "v0.1.5 is released",
		Note:         "v0.1.5 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}
	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &release, []string{attach.UUID}, "test"))
}

func TestRelease_Update(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	gitRepo, err := git.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Advance a mocked clock between create and update instead of sleeping, so the
	// timestamp-sensitive assertions below stay deterministic.
	fakeNow := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	defer timeutil.MockSet(fakeNow)()
	advance := func() { fakeNow = fakeNow.Add(time.Second); timeutil.MockSet(fakeNow) }

	// Test a changed release
	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.1.1",
		Target:       "master",
		Title:        "v1.1.1 is released",
		Note:         "v1.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))
	release, err := repo_model.GetRelease(t.Context(), repo.ID, "v1.1.1")
	assert.NoError(t, err)
	releaseCreatedUnix := release.CreatedUnix
	releasePublishedUnix := release.PublishedUnix
	advance()
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))
	assert.Equal(t, releasePublishedUnix, release.PublishedUnix, "editing does not republish")

	// Test a changed draft
	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.2.1",
		Target:       "65f1bf2",
		Title:        "v1.2.1 is draft",
		Note:         "v1.2.1 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))
	release, err = repo_model.GetRelease(t.Context(), repo.ID, "v1.2.1")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	advance()
	release.Title = "Changed title"
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Less(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))
	assert.Zero(t, release.PublishedUnix, "a draft is unpublished")

	// Test publishing and withdrawing that draft
	release.IsDraft = false
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.NotZero(t, release.PublishedUnix, "publishing stamps the publication time")
	release.IsDraft = true
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Zero(t, release.PublishedUnix, "withdrawing unpublishes it again")

	// Test a changed pre-release
	assert.NoError(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.3.1",
		Target:       "65f1bf2",
		Title:        "v1.3.1 is pre-released",
		Note:         "v1.3.1 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil, ""))
	release, err = repo_model.GetRelease(t.Context(), repo.ID, "v1.3.1")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	advance()
	release.Title = "Changed title"
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test create release
	release = &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.1.2",
		Target:       "",
		Title:        "v1.1.2 is released",
		Note:         "v1.1.2 is released",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}
	assert.NoError(t, CreateRelease(t.Context(), gitRepo, release, nil, ""))
	assert.Positive(t, release.ID)

	release.IsDraft = false
	tagName := release.TagName

	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Equal(t, tagName, release.TagName)

	// Add new attachments
	samplePayload := "testtest"
	attach, err := attachment.NewAttachment(t.Context(), &repo_model.Attachment{
		RepoID:     repo.ID,
		UploaderID: user.ID,
		Name:       "test.txt",
	}, strings.NewReader(samplePayload), int64(len([]byte(samplePayload))))
	assert.NoError(t, err)

	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, []string{attach.UUID}, nil, nil))
	assert.NoError(t, repo_model.GetReleaseAttachments(t.Context(), release))
	assert.Len(t, release.Attachments, 1)
	assert.Equal(t, attach.UUID, release.Attachments[0].UUID)
	assert.Equal(t, release.ID, release.Attachments[0].ReleaseID)
	assert.Equal(t, attach.Name, release.Attachments[0].Name)

	// update the attachment name
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, map[string]string{
		attach.UUID: "test2.txt",
	}))
	release.Attachments = nil
	assert.NoError(t, repo_model.GetReleaseAttachments(t.Context(), release))
	assert.Len(t, release.Attachments, 1)
	assert.Equal(t, attach.UUID, release.Attachments[0].UUID)
	assert.Equal(t, release.ID, release.Attachments[0].ReleaseID)
	assert.Equal(t, "test2.txt", release.Attachments[0].Name)

	defer test.MockVariableValue(&setting.Repository.Release.AllowedTypes, ".zip")()
	err = UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, map[string]string{
		attach.UUID: "test.exe",
	})
	assert.Error(t, err)
	assert.True(t, upload.IsErrFileTypeForbidden(err))
	release.Attachments = nil
	assert.NoError(t, repo_model.GetReleaseAttachments(t.Context(), release))
	assert.Len(t, release.Attachments, 1)
	assert.Equal(t, "test2.txt", release.Attachments[0].Name)

	// delete the attachment
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, []string{attach.UUID}, nil))
	release.Attachments = nil
	assert.NoError(t, repo_model.GetReleaseAttachments(t.Context(), release))
	assert.Empty(t, release.Attachments)
}

func TestRelease_createTag(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	gitRepo, err := git.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Advance a mocked clock between create and update instead of sleeping, so the
	// timestamp-sensitive assertions below stay deterministic.
	fakeNow := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	defer timeutil.MockSet(fakeNow)()
	advance := func() { fakeNow = fakeNow.Add(time.Second); timeutil.MockSet(fakeNow) }

	// Test a changed release
	release := &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.1.1",
		Target:       "master",
		Title:        "v2.1.1 is released",
		Note:         "v2.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	assert.NotEmpty(t, release.CreatedUnix)
	releaseCreatedUnix := release.CreatedUnix
	advance()
	release.Note = "Changed note"
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed draft
	release = &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.2.1",
		Target:       "65f1bf2",
		Title:        "v2.2.1 is draft",
		Note:         "v2.2.1 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	advance()
	release.Title = "Changed title"
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	assert.Less(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed pre-release
	release = &repo_model.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.3.1",
		Target:       "65f1bf2",
		Title:        "v2.3.1 is pre-released",
		Note:         "v2.3.1 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	advance()
	release.Title = "Changed title"
	release.Note = "Changed note"
	_, err = createTag(t.Context(), gitRepo, release, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))
}

func TestCreateNewTag(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	assert.NoError(t, CreateNewTag(t.Context(), user, repo, "master", "v2.0",
		"v2.0 is released \n\n BUGFIX: .... \n\n 123"))
}

func TestRelease_DatedByTargetCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	newRelease := func(tagName, target string) *repo_model.Release {
		rel := &repo_model.Release{
			RepoID: repo.ID, Repo: repo, PublisherID: user.ID, Publisher: user,
			TagName: tagName, Target: target, Title: tagName,
		}
		assert.NoError(t, CreateRelease(t.Context(), gitRepo, rel, nil, ""))
		return rel
	}

	recent := newRelease("v9.9-recent", "DefaultBranch")
	// released afterwards, but from an older commit, so it must not take over as the latest release
	old := newRelease("v9.9-old", "master")

	oldCommit, err := gitRepo.GetBranchCommit(t.Context(), "master")
	assert.NoError(t, err)
	assert.Equal(t, oldCommit.Committer.When.Unix(), int64(old.CreatedUnix), "a release is dated by the commit it points at")
	assert.Greater(t, int64(old.PublishedUnix), int64(old.CreatedUnix), "but its publication time is now")

	latest, err := repo_model.GetLatestReleaseByRepoID(t.Context(), repo.ID)
	assert.NoError(t, err)
	assert.Equal(t, recent.ID, latest.ID)
}
