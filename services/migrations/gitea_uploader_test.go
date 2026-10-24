// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/graceful"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/structs"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestGiteaUploadRepo(t *testing.T) {
	// FIXME: Since no accesskey or user/password will trigger rate limit of github, just skip
	t.Skip()

	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	ctx := t.Context()
	downloader, err := NewGithubDownloaderV3(ctx, "https://github.com", "", "", "", "go-xorm", "builder")
	require.NoError(t, err)
	var (
		repoName = "builder-" + time.Now().Format("2006-01-02-15-04-05")
		uploader = NewGiteaLocalUploader(graceful.GetManager().HammerContext(), user, user.Name, repoName)
	)

	err = migrateRepository(t.Context(), user, downloader, uploader, base.MigrateOptions{
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

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName})
	assert.True(t, repo_service.HasWiki(ctx, repo))
	assert.Equal(t, repo_model.RepositoryReady, repo.Status)

	milestones, err := db.Find[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(false),
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 1)

	milestones, err = db.Find[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(true),
	})
	assert.NoError(t, err)
	assert.Empty(t, milestones)

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, labels, 12)

	releases, err := db.Find[repo_model.Release](t.Context(), repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: true,
		RepoID:      repo.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 8)

	releases, err = db.Find[repo_model.Release](t.Context(), repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: false,
		RepoID:      repo.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	issues, err := issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsPull:   optional.Some(false),
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 15)
	assert.NoError(t, issues[0].LoadDiscussComments(t.Context()))
	assert.Empty(t, issues[0].Comments)

	pulls, _, err := issues_model.PullRequests(t.Context(), repo.ID, &issues_model.PullRequestsOptions{
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, pulls, 30)
	assert.NoError(t, pulls[0].LoadIssue(t.Context()))
	assert.NoError(t, pulls[0].Issue.LoadDiscussComments(t.Context()))
	assert.Len(t, pulls[0].Issue.Comments, 2)
}

func TestCreateReleasesSkipsUnavailableAssets(t *testing.T) {
	unittest.PrepareTestEnv(t)
	sourceRelease := &base.Release{Name: "assets", Assets: []*base.ReleaseAsset{
		{Name: "gone", Size: new(0), DownloadCount: new(0), DownloadFunc: func() (io.ReadCloser, error) { return nil, errors.New("gone") }},
		{Name: "kept", Size: new(0), DownloadCount: new(0), DownloadFunc: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("data")), nil }},
	}}

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	uploader := NewGiteaLocalUploader(t.Context(), doer, doer.Name, "repo1")
	uploader.repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, uploader.CreateReleases(t.Context(), sourceRelease))
	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: 1, Title: "assets"})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ReleaseID: release.ID, Name: "kept", Size: 4})
	unittest.AssertNotExistsBean(t, &repo_model.Attachment{ReleaseID: release.ID, Name: "gone"})

	dumper := &RepositoryDumper{baseDir: t.TempDir(), opts: base.MigrateOptions{ReleaseAssets: true}}
	defer dumper.Close()
	require.NoError(t, dumper.CreateReleases(t.Context(), sourceRelease))
	require.Len(t, sourceRelease.Assets, 1)
	assert.Equal(t, "kept", sourceRelease.Assets[0].Name)
}

func TestGiteaUploadRemapLocalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	ctx := t.Context()
	repoName := "migrated"
	uploader := NewGiteaLocalUploader(ctx, doer, doer.Name, repoName)
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
	target := repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err := uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.Equal(t, doer.ID, target.GetUserID())

	//
	// The externalID matches a known user but the name does not match,
	// everything belongs to the doer
	//
	source.PublisherID = user.ID
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.Equal(t, doer.ID, target.GetUserID())

	//
	// The externalID and externalName match an existing user, everything
	// belongs to the existing user
	//
	source.PublisherName = user.Name
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, target.GetUserID())
}

func TestGiteaUploadRemapExternalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	ctx := t.Context()
	repoName := "migrated"
	uploader := NewGiteaLocalUploader(ctx, doer, doer.Name, repoName)
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
	target := repo_model.Release{}
	err := uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.Equal(t, doer.ID, target.GetUserID())

	//
	// Link the external ID to an existing user
	//
	linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    strconv.FormatInt(externalID, 10),
		UserID:        linkedUser.ID,
		LoginSourceID: 0,
		Provider:      structs.GiteaService.Name(),
	}
	err = user_model.LinkExternalToUser(t.Context(), linkedUser, externalLoginUser)
	assert.NoError(t, err)

	//
	// When a user is linked to the external ID, it becomes the author of
	// the migrated data
	//
	uploader.userMap = make(map[int64]int64)
	target = repo_model.Release{}
	err = uploader.remapUser(ctx, &source, &target)
	assert.NoError(t, err)
	assert.Equal(t, linkedUser.ID, target.GetUserID())
}

func TestGiteaUploadReviewCommentResolved(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	require.NoError(t, pull.LoadIssue(ctx))

	uploader := NewGiteaLocalUploader(ctx, doer, repo.OwnerName, repo.Name)
	var err error
	uploader.repo = repo
	uploader.gitRepo, err = git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer uploader.Close()
	uploader.issues[pull.Issue.Index] = pull.Issue

	headCommitID, err := uploader.gitRepo.GetBranchCommitID(ctx, repo.DefaultBranch)
	require.NoError(t, err)
	require.NoError(t, git.UpdateRef(ctx, repo, pull.GetGitHeadRefName(), headCommitID))

	require.NoError(t, uploader.CreateReviews(ctx, &base.Review{
		IssueIndex: pull.Issue.Index,
		ReviewerID: doer.ID,
		State:      base.ReviewStateCommented,
		Comments: []*base.ReviewComment{
			{Content: "resolved thread", TreePath: "README.md", Line: 1, Resolved: true},
			{Content: "open thread", TreePath: "README.md", Line: 1},
		},
	}))

	comments, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{IssueID: pull.Issue.ID, Type: issues_model.CommentTypeCode})
	require.NoError(t, err)
	resolved := map[string]bool{}
	for _, c := range comments {
		if c.Content == "resolved thread" || c.Content == "open thread" {
			resolved[c.Content] = c.ResolveDoerID == doer.ID
		}
	}
	assert.Equal(t, map[string]bool{"resolved thread": true, "open thread": false}, resolved)
}

func TestGiteaUploadForkPullRequestHeadInRepository(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	uploader := NewGiteaLocalUploader(ctx, doer, repo.OwnerName, repo.Name)
	var err error
	uploader.repo = repo
	uploader.gitRepo, err = git.OpenRepository(ctx, repo)
	require.NoError(t, err)
	defer uploader.Close()

	headCommitID, err := uploader.gitRepo.GetBranchCommitID(ctx, repo.DefaultBranch)
	require.NoError(t, err)

	// the fork cannot be reached, but its head commit is already in the repository
	pr := &base.PullRequest{
		Number: 99,
		State:  "open",
		Head:   base.PullRequestBranch{OwnerName: "fork-owner", RepoName: "repo", Ref: "feature", SHA: headCommitID, CloneURL: "https://fork.invalid/fork-owner/repo.git"},
		Base:   base.PullRequestBranch{OwnerName: repo.OwnerName, RepoName: repo.Name, Ref: repo.DefaultBranch},
	}
	pr.EnsuredSafe = true
	require.True(t, pr.IsForkPullRequest())

	head, err := uploader.updateGitForPullRequest(ctx, pr)
	require.NoError(t, err)
	assert.Equal(t, "fork-owner/feature", head)
	assert.True(t, uploader.gitRepo.IsBranchExist(ctx, "fork-owner/feature"))
	branchCommitID, err := uploader.gitRepo.GetBranchCommitID(ctx, "fork-owner/feature")
	require.NoError(t, err)
	assert.Equal(t, headCommitID, branchCommitID)
	refCommitID, err := uploader.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	require.NoError(t, err)
	assert.Equal(t, headCommitID, refCommitID)
	remotes, _, err := gitcmd.NewCommand("remote").WithRepo(repo).RunStdString(ctx)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(remotes), "no remote is added for a local head")
}

func TestGiteaUploadTimelineLabelAndMilestoneComments(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})

	ctx := t.Context()
	uploader := NewGiteaLocalUploader(ctx, doer, doer.Name, "migrated")
	uploader.issues[issue.Index] = issue
	uploader.labels[label.Name] = label
	uploader.milestones[milestone.Name] = milestone.ID

	require.NoError(t, uploader.CreateComments(ctx,
		&base.Comment{IssueIndex: issue.Index, CommentType: "label", Content: "1", Meta: map[string]any{"LabelName": label.Name}},
		&base.Comment{IssueIndex: issue.Index, CommentType: "label", Meta: map[string]any{"LabelName": "deleted-upstream"}},
		&base.Comment{IssueIndex: issue.Index, CommentType: "milestone", Meta: map[string]any{"MilestoneTitle": milestone.Name}},
		&base.Comment{IssueIndex: issue.Index, CommentType: "milestone", Meta: map[string]any{"MilestoneTitle": milestone.Name, "Removed": true}},
		&base.Comment{IssueIndex: issue.Index, CommentType: "delete_branch", Content: "feature"},
	))

	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeLabel, LabelID: label.ID, Content: "1"})
	unittest.AssertCountByCond(t, "comment", builder.Eq{
		"issue_id": issue.ID,
		"type":     issues_model.CommentTypeLabel,
		"label_id": 0,
	}, 0)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeMilestone, MilestoneID: milestone.ID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeMilestone, OldMilestoneID: milestone.ID})
	assert.Empty(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeDeleteBranch, OldRef: "feature"}).Content)
}
