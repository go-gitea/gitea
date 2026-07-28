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

	gitRepo, err := git.OpenRepository(repo)
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

	gitRepo, err := git.OpenRepository(repo)
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
	advance()
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, release, nil, nil, nil))
	release, err = repo_model.GetReleaseByID(t.Context(), release.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

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
		Target:       "master",
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

	gitRepo, err := git.OpenRepository(repo)
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

func TestRelease_Immutable(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	gitRepo, err := git.OpenRepository(repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	newRelease := func(t *testing.T, tagName string) *repo_model.Release {
		rel := &repo_model.Release{
			RepoID: repo.ID, Repo: repo, PublisherID: user.ID, Publisher: user,
			TagName: tagName, Target: "master", Title: tagName + " is released",
		}
		assert.NoError(t, CreateRelease(t.Context(), gitRepo, rel, nil, ""))
		return rel
	}

	t.Run("NotAppliedRetroactively", func(t *testing.T) {
		repo.ImmutableReleases = false
		rel := newRelease(t, "v9.6")
		assert.False(t, rel.IsImmutable)

		// enabling the setting must not lock releases that are already published
		repo.ImmutableReleases = true
		rel.Note = "typo fixed"
		assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, rel, nil, nil, nil))
		assert.False(t, rel.IsImmutable)

		immutable, err := repo_model.IsTagImmutable(t.Context(), repo, "v9.6")
		assert.NoError(t, err)
		assert.False(t, immutable)
	})

	repo.ImmutableReleases = true
	rel := newRelease(t, "v9.0")

	t.Run("StampedOnPublish", func(t *testing.T) {
		assert.True(t, rel.IsImmutable)
		immutable, err := repo_model.IsTagImmutable(t.Context(), repo, "v9.0")
		assert.NoError(t, err)
		assert.True(t, immutable)
	})

	t.Run("TitleAndNotesStayEditable", func(t *testing.T) {
		rel.Title = "changed title"
		rel.Note = "changed note"
		assert.NoError(t, UpdateRelease(t.Context(), user, gitRepo, rel, nil, nil, nil))
	})

	// one case per rejected field, the three attachment slices share a single branch
	lockedCases := []struct {
		name   string
		mutate func(rel *repo_model.Release)
		attach []string
	}{
		{name: "tag_name", mutate: func(rel *repo_model.Release) { rel.TagName = "v9.1" }},
		{name: "target_commitish", mutate: func(rel *repo_model.Release) { rel.Target = "develop" }},
		{name: "state", mutate: func(rel *repo_model.Release) { rel.IsDraft = true }},
		{name: "assets", attach: []string{"uuid"}},
	}
	for _, c := range lockedCases {
		t.Run("Locked/"+c.name, func(t *testing.T) {
			current, err := repo_model.GetReleaseByID(t.Context(), rel.ID)
			assert.NoError(t, err)
			current.Repo = repo
			if c.mutate != nil {
				c.mutate(current)
			}
			err = UpdateRelease(t.Context(), user, gitRepo, current, c.attach, nil, nil)
			assert.True(t, IsErrImmutableRelease(err), "expected ErrImmutableRelease, got %v", err)
		})
	}

	t.Run("TagLifecycle", func(t *testing.T) {
		rel := newRelease(t, "v9.2")

		// the tag cannot be deleted while the release exists
		err := DeleteReleaseByID(t.Context(), repo, rel, user, true)
		assert.ErrorIs(t, err, ErrImmutableTag)

		// deleting the release itself is allowed, the tag remains as a locked tag
		assert.NoError(t, DeleteReleaseByID(t.Context(), repo, rel, user, false))
		tag, err := repo_model.GetRelease(t.Context(), repo.ID, "v9.2")
		assert.NoError(t, err)
		assert.True(t, tag.IsTag)
		assert.True(t, tag.IsImmutable)

		// it cannot be turned back into a release, not even a draft
		tag.Repo, tag.IsTag, tag.IsDraft = repo, false, true
		err = UpdateRelease(t.Context(), user, gitRepo, tag, nil, nil, nil)
		assert.ErrorIs(t, err, ErrImmutableTag)

		// once the release is gone the tag itself can be deleted, but the name stays claimed
		tag, err = repo_model.GetRelease(t.Context(), repo.ID, "v9.2")
		assert.NoError(t, err)
		tag.Repo = repo
		assert.NoError(t, DeleteReleaseByID(t.Context(), repo, tag, user, true))
		assert.ErrorIs(t, CreateRelease(t.Context(), gitRepo, &repo_model.Release{
			RepoID: repo.ID, Repo: repo, PublisherID: user.ID, Publisher: user,
			TagName: "v9.2", Target: "master", Title: "reuse",
		}, nil, ""), ErrImmutableTag)
	})
}
