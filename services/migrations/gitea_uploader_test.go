// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"strconv"
	"testing"
	"time"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/graceful"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGiteaUploadIssueMetadata(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assignee := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	uploader := NewGiteaLocalUploader(ctx, doer, repo.OwnerName, repo.Name)
	uploader.repo = repo
	uploader.sameApp = true

	closedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	reactionAt := time.Date(2026, 2, 2, 3, 4, 5, 0, time.UTC)
	beforeFallback := timeutil.TimeStampNow()
	require.NoError(t, uploader.CreateIssues(ctx,
		&base.Issue{
			Number: 9001, PosterID: doer.ID, PosterName: doer.Name, Title: "metadata", State: "closed",
			Created: closedAt.Add(-time.Hour), Updated: closedAt, Closed: &closedAt,
			ClosedBy: &base.ExternalUser{ID: assignee.ID, Name: assignee.Name}, CloseReason: "not_planned",
			AssigneeUsers: []*base.ExternalUser{{ID: assignee.ID, Name: assignee.Name}, {ID: 999999, Name: "missing"}},
			Reactions: []*base.Reaction{
				{UserID: assignee.ID, UserName: assignee.Name, Content: "+1", Created: reactionAt},
				{UserID: assignee.ID, UserName: assignee.Name, Content: "heart"},
			},
		},
		&base.Issue{
			Number: 9002, PosterID: doer.ID, PosterName: doer.Name, Title: "no actor", State: "closed",
			Created: closedAt.Add(-time.Hour), Updated: closedAt, Closed: &closedAt,
		},
		&base.Issue{
			Number: 9005, PosterID: doer.ID, PosterName: doer.Name, Title: "external actor", State: "closed",
			Created: closedAt.Add(-time.Hour), Updated: closedAt, Closed: &closedAt,
			ClosedBy: &base.ExternalUser{ID: 999998, Name: "external-closer"}, CloseReason: "completed",
		},
		&base.Issue{
			Number: 9006, PosterID: doer.ID, PosterName: doer.Name, Title: "no close time", State: "closed",
			Created: closedAt.Add(-time.Hour), Updated: closedAt,
			ClosedBy: &base.ExternalUser{ID: assignee.ID, Name: assignee.Name}, CloseReason: "completed",
		},
	))
	afterFallback := timeutil.TimeStampNow()

	var issue issues_model.Issue
	has, err := db.GetEngine(ctx).Where("repo_id = ? AND `index` = ?", repo.ID, 9001).Get(&issue)
	require.NoError(t, err)
	require.True(t, has)
	assert.True(t, issue.IsClosed)
	assert.Equal(t, timeutil.TimeStamp(closedAt.Unix()), issue.ClosedUnix)
	assigneeIDs, err := issues_model.GetAssigneeIDsByIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{assignee.ID}, assigneeIDs)

	var reactions []*issues_model.Reaction
	require.NoError(t, db.GetEngine(ctx).Where("issue_id = ?", issue.ID).OrderBy("type").Find(&reactions))
	require.Len(t, reactions, 2)
	assert.Equal(t, timeutil.TimeStamp(reactionAt.Unix()), reactions[0].CreatedUnix)
	assert.GreaterOrEqual(t, reactions[1].CreatedUnix, beforeFallback)
	assert.LessOrEqual(t, reactions[1].CreatedUnix, afterFallback)

	closeComments, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{IssueID: issue.ID, Type: issues_model.CommentTypeClose})
	require.NoError(t, err)
	require.Len(t, closeComments, 1)
	closeComment := closeComments[0]
	assert.Equal(t, assignee.ID, closeComment.PosterID)
	assert.Equal(t, timeutil.TimeStamp(closedAt.Unix()), closeComment.CreatedUnix)
	require.NotNil(t, closeComment.CommentMetaData)
	assert.Equal(t, "not_planned", closeComment.CommentMetaData.CloseReason)

	missingActorIssue := uploader.issues[9002]
	has, err = db.GetEngine(ctx).Where("issue_id = ? AND type = ?", missingActorIssue.ID, issues_model.CommentTypeClose).Exist(new(issues_model.Comment))
	require.NoError(t, err)
	assert.False(t, has)
	externalActorIssue := uploader.issues[9005]
	var externalCloseComment issues_model.Comment
	has, err = db.GetEngine(ctx).Where("issue_id = ? AND type = ?", externalActorIssue.ID, issues_model.CommentTypeClose).Get(&externalCloseComment)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, doer.ID, externalCloseComment.PosterID)
	assert.Equal(t, "external-closer", externalCloseComment.OriginalAuthor)
	assert.Equal(t, int64(999998), externalCloseComment.OriginalAuthorID)
	assert.Equal(t, timeutil.TimeStamp(closedAt.Unix()), externalCloseComment.CreatedUnix)
	missingTimeIssue := uploader.issues[9006]
	has, err = db.GetEngine(ctx).Where("issue_id = ? AND type = ?", missingTimeIssue.ID, issues_model.CommentTypeClose).Exist(new(issues_model.Comment))
	require.NoError(t, err)
	assert.False(t, has)

	commentReactionAt := reactionAt.Add(time.Minute)
	require.NoError(t, uploader.CreateComments(ctx, &base.Comment{
		IssueIndex: 9001, PosterID: doer.ID, PosterName: doer.Name, Content: "comment", Created: closedAt,
		Reactions: []*base.Reaction{{UserID: assignee.ID, UserName: assignee.Name, Content: "rocket", Created: commentReactionAt}},
	}))
	var commentReaction issues_model.Reaction
	has, err = db.GetEngine(ctx).Where("issue_id = ? AND type = ?", issue.ID, "rocket").Get(&commentReaction)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, timeutil.TimeStamp(commentReactionAt.Unix()), commentReaction.CreatedUnix)
}

func TestGiteaUploadPullRequestMetadata(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	merger := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	uploader := NewGiteaLocalUploader(ctx, doer, repo.OwnerName, repo.Name)
	uploader.repo = repo
	uploader.sameApp = true

	mergedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	reactionAt := mergedAt.Add(-time.Minute)
	source := &base.PullRequest{
		Number: 9003, PosterID: doer.ID, PosterName: doer.Name, Title: "merged", State: "closed",
		Created: mergedAt.Add(-time.Hour), Updated: mergedAt, Closed: &mergedAt, Merged: true, MergedTime: &mergedAt,
		MergedBy: &base.ExternalUser{ID: merger.ID, Name: merger.Name}, MergeCommitSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AssigneeUsers: []*base.ExternalUser{{ID: merger.ID, Name: merger.Name}, {ID: 999999, Name: "missing"}},
		Reactions:     []*base.Reaction{{UserID: merger.ID, UserName: merger.Name, Content: "heart", Created: reactionAt}},
		Head:          base.PullRequestBranch{Ref: "topic", OwnerName: repo.OwnerName, RepoName: repo.Name},
		Base:          base.PullRequestBranch{Ref: "main", SHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", OwnerName: repo.OwnerName, RepoName: repo.Name},
		EnsuredSafe:   true,
	}
	pullRequest, err := uploader.newPullRequest(ctx, source)
	require.NoError(t, err)
	assert.Equal(t, merger.ID, pullRequest.MergerID)
	require.Len(t, pullRequest.Issue.Assignees, 1)
	assert.Equal(t, merger.ID, pullRequest.Issue.Assignees[0].ID)
	require.Len(t, pullRequest.Issue.Reactions, 1)
	assert.Equal(t, timeutil.TimeStamp(reactionAt.Unix()), pullRequest.Issue.Reactions[0].CreatedUnix)

	source.MergedBy = &base.ExternalUser{ID: 999999, Name: "missing"}
	pullRequest, err = uploader.newPullRequest(ctx, source)
	require.NoError(t, err)
	assert.Equal(t, user_model.GhostUserID, pullRequest.MergerID)
	source.MergedBy = nil
	pullRequest, err = uploader.newPullRequest(ctx, source)
	require.NoError(t, err)
	assert.Equal(t, user_model.GhostUserID, pullRequest.MergerID)
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
