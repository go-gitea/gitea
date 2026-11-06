// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"context"
	"sync/atomic"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
	"xorm.io/xorm/contexts"
)

type statementCounter struct{ count atomic.Int64 }

func (c *statementCounter) BeforeProcess(h *contexts.ContextHook) (context.Context, error) {
	c.count.Add(1)
	return h.Ctx, nil
}

func (*statementCounter) AfterProcess(*contexts.ContextHook) error { return nil }

// countStatements runs fn and returns how many SQL statements it executed.
func countStatements(t *testing.T, fn func()) int64 {
	counter := &statementCounter{}
	unittest.GetXORMEngine().AddHook(counter)
	fn()
	n := counter.count.Load()
	counter.count.Store(-1 << 40) // the hook cannot be removed, make it harmless for later measurements
	return n
}

var nextTestIssueIndex int64 = 9000

func insertTestIssue(t *testing.T, repo *repo_model.Repository, poster *user_model.User, confidential bool) *issues_model.Issue {
	nextTestIssueIndex++
	issue := &issues_model.Issue{
		RepoID: repo.ID, Repo: repo, Index: nextTestIssueIndex, PosterID: poster.ID, Poster: poster,
		Title: "confidential test", IsConfidential: confidential,
	}
	require.NoError(t, db.Insert(t.Context(), issue))
	return issue
}

func doerPerm(t *testing.T, repo *repo_model.Repository, doer *user_model.User) access_model.Permission {
	p, err := access_model.GetDoerRepoPermission(t.Context(), repo, doer)
	require.NoError(t, err)
	return p
}

func doerPermPtr(t *testing.T, repo *repo_model.Repository, doer *user_model.User) *access_model.Permission {
	p := doerPerm(t, repo, doer)
	return &p
}

type confidentialActors struct {
	repo                                                    *repo_model.Repository
	owner, writer, reader, reporter, assignee, other, admin *user_model.User
	repoAdmin, publicOnlyAdmin                              *user_model.User
	reporterIssue, readerIssue                              *issues_model.Issue
	publicIssue                                             *issues_model.Issue
}

func setupConfidentialActors(t *testing.T) *confidentialActors {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	a := &confidentialActors{
		repo:      unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}),
		owner:     unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}),
		writer:    unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}),
		reader:    unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}),
		reporter:  unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10}),
		assignee:  unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8}),
		other:     unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9}),
		admin:     unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}),
		repoAdmin: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11}),
	}
	require.NoError(t, repo_service.AddOrUpdateCollaborator(ctx, a.repo, a.writer, perm.AccessModeWrite))
	require.NoError(t, repo_service.AddOrUpdateCollaborator(ctx, a.repo, a.reader, perm.AccessModeRead))
	require.NoError(t, repo_service.AddOrUpdateCollaborator(ctx, a.repo, a.repoAdmin, perm.AccessModeAdmin))

	publicOnly := *a.admin
	publicOnly.ExtDoerData = user_model.NewTokenExtDoerData(user_model.CredentialAccessToken, 1, "public-only,read:issue")
	a.publicOnlyAdmin = &publicOnly

	assert.True(t, doerPermPtr(t, a.repo, a.owner).CanWriteIssuesOrPulls(false))
	assert.True(t, doerPermPtr(t, a.repo, a.writer).CanWriteIssuesOrPulls(false))
	assert.True(t, doerPermPtr(t, a.repo, a.repoAdmin).IsAdmin())
	for _, u := range []*user_model.User{a.reader, a.reporter, a.assignee, a.other, nil} {
		p := doerPermPtr(t, a.repo, u)
		assert.True(t, p.CanReadIssuesOrPulls(false))
		assert.False(t, p.CanWriteIssuesOrPulls(false))
	}

	a.reporterIssue = insertTestIssue(t, a.repo, a.reporter, true)
	a.readerIssue = insertTestIssue(t, a.repo, a.reader, true)
	a.publicIssue = insertTestIssue(t, a.repo, a.reporter, false)
	require.NoError(t, db.Insert(ctx, &issues_model.IssueAssignees{IssueID: a.reporterIssue.ID, AssigneeID: a.assignee.ID}))
	return a
}

func isVisible(t *testing.T, issue *issues_model.Issue, doer *user_model.User) bool {
	visible, err := issues_model.IsIssueVisible(t.Context(), issue, doer, doerPerm(t, issue.Repo, doer))
	require.NoError(t, err)
	return visible
}

func TestIsIssueVisibleConfidentialMatrix(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()

	for name, tc := range map[string]struct {
		doer  *user_model.User
		issue *issues_model.Issue
		want  bool
	}{
		"reporter":                  {a.reporter, a.reporterIssue, true},
		"author who is only reader": {a.reader, a.readerIssue, true},
		"reader on foreign issue":   {a.reader, a.reporterIssue, false},
		"assignee":                  {a.assignee, a.reporterIssue, true},
		"writer":                    {a.writer, a.reporterIssue, true},
		"repo admin":                {a.repoAdmin, a.reporterIssue, true},
		"owner":                     {a.owner, a.reporterIssue, true},
		"site admin":                {a.admin, a.reporterIssue, true},
		"site admin public-only":    {a.publicOnlyAdmin, a.reporterIssue, false},
		"signed-in other":           {a.other, a.reporterIssue, false},
		"anonymous":                 {nil, a.reporterIssue, false},
		"anonymous on public issue": {nil, a.publicIssue, true},
		"other on public issue":     {a.other, a.publicIssue, true},
		"public-only admin, public": {a.publicOnlyAdmin, a.publicIssue, true},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isVisible(t, tc.issue, tc.doer))
		})
	}

	t.Run("assignee after unassign", func(t *testing.T) {
		_, err := db.GetEngine(ctx).Delete(&issues_model.IssueAssignees{IssueID: a.reporterIssue.ID, AssigneeID: a.assignee.ID})
		require.NoError(t, err)
		assert.False(t, isVisible(t, a.reporterIssue, a.assignee))
	})

	t.Run("org owner", func(t *testing.T) {
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		issue := insertTestIssue(t, repo3, a.reporter, true)
		assert.True(t, doerPermPtr(t, repo3, a.owner).IsOwner() || doerPermPtr(t, repo3, a.owner).IsAdmin())
		assert.True(t, isVisible(t, issue, a.owner))
	})

	t.Run("poster without repo read", func(t *testing.T) {
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		issue := insertTestIssue(t, repo3, a.reporter, true)
		assert.False(t, doerPermPtr(t, repo3, a.reporter).CanReadIssuesOrPulls(false))
		assert.False(t, isVisible(t, issue, a.reporter))
	})

	t.Run("pull request", func(t *testing.T) {
		pull := insertTestIssue(t, a.repo, a.reporter, false)
		pull.IsPull = true
		assert.True(t, isVisible(t, pull, a.other))
		assert.True(t, isVisible(t, pull, nil))
	})

	t.Run("CheckIssueVisible", func(t *testing.T) {
		err := issues_model.CheckIssueVisible(ctx, a.readerIssue, a.other, doerPerm(t, a.repo, a.other))
		assert.Equal(t, issues_model.ErrIssueNotExist{ID: 0, RepoID: a.repo.ID, Index: a.readerIssue.Index}, err)
		assert.ErrorIs(t, err, util.ErrNotExist)
		assert.NoError(t, issues_model.CheckIssueVisible(ctx, a.readerIssue, a.writer, doerPerm(t, a.repo, a.writer)))
	})
}

func TestVisibilityScopeCondMatchesIsIssueVisibleConfidential(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	issueIDs := []int64{a.reporterIssue.ID, a.readerIssue.ID, a.publicIssue.ID}

	for name, doer := range map[string]*user_model.User{
		"reporter": a.reporter, "reader": a.reader, "assignee": a.assignee, "writer": a.writer,
		"repo admin": a.repoAdmin, "owner": a.owner, "admin": a.admin, "public-only admin": a.publicOnlyAdmin,
		"other": a.other, "anonymous": nil,
	} {
		t.Run(name, func(t *testing.T) {
			scope, err := issues_model.BuildVisibilityScope(ctx, doer, []int64{a.repo.ID}, false)
			require.NoError(t, err)
			issues, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{IssueIDs: issueIDs, Visibility: scope})
			require.NoError(t, err)
			var got, want []int64
			for _, issue := range issues {
				got = append(got, issue.ID)
			}
			for _, issue := range []*issues_model.Issue{a.reporterIssue, a.readerIssue, a.publicIssue} {
				if isVisible(t, issue, doer) {
					want = append(want, issue.ID)
				}
			}
			assert.ElementsMatch(t, want, got)
		})
	}
}

func TestBuildVisibilityScopeConfidential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})

	var scope *issues_model.VisibilityScope
	statements := countStatements(t, func() {
		var err error
		scope, err = issues_model.BuildVisibilityScope(ctx, other, []int64{repo.ID}, false)
		require.NoError(t, err)
	})
	assert.EqualValues(t, 1, statements)
	assert.Empty(t, scope.WriterRepoIDs)
	assert.Equal(t, other.ID, scope.DoerID)

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	scope, err := issues_model.BuildVisibilityScope(ctx, admin, []int64{repo.ID}, false)
	require.NoError(t, err)
	assert.True(t, scope.Unrestricted)

	publicOnly := *admin
	publicOnly.ExtDoerData = user_model.NewTokenExtDoerData(user_model.CredentialAccessToken, 1, "public-only,read:issue")
	scope, err = issues_model.BuildVisibilityScope(ctx, &publicOnly, []int64{repo.ID}, false)
	require.NoError(t, err)
	assert.False(t, scope.Unrestricted)
	assert.Zero(t, scope.DoerID)
}

func TestIssuesOptionsResolveVisibilityConfidential(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	opts := func(doer *user_model.User) *issues_model.IssuesOptions {
		return &issues_model.IssuesOptions{IssueIDs: []int64{a.reporterIssue.ID}, Doer: doer}
	}

	issues, err := issues_model.Issues(ctx, opts(nil))
	require.NoError(t, err)
	assert.Empty(t, issues)

	issues, err = issues_model.Issues(ctx, opts(a.writer))
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	count, err := issues_model.CountIssues(ctx, opts(a.other))
	require.NoError(t, err)
	assert.Zero(t, count)

	stats, err := issues_model.GetIssueStats(ctx, opts(a.writer))
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.OpenCount)

	confidentialOnly := opts(a.writer)
	confidentialOnly.IssueIDs = []int64{a.reporterIssue.ID, a.publicIssue.ID}
	confidentialOnly.Confidential = optional.Some(true)
	ids, _, err := issues_model.IssueIDs(ctx, confidentialOnly)
	require.NoError(t, err)
	assert.Equal(t, []int64{a.reporterIssue.ID}, ids)
}

func TestSetIssueConfidential(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	issue := a.publicIssue
	opts := &issues_model.IssueConfidentialOptions{Doer: a.writer, Issue: issue}

	comment, err := issues_model.SetIssueConfidential(ctx, opts, true)
	require.NoError(t, err)
	require.NotNil(t, comment)
	assert.Equal(t, issues_model.CommentTypeMakeConfidential, comment.Type)
	assert.Equal(t, a.writer.ID, comment.PosterID)
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)

	comment, err = issues_model.SetIssueConfidential(ctx, opts, true)
	require.NoError(t, err)
	assert.Nil(t, comment)
	unittest.AssertCount(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeMakeConfidential}, 1)

	comment, err = issues_model.SetIssueConfidential(ctx, opts, false)
	require.NoError(t, err)
	assert.Equal(t, issues_model.CommentTypeMakePublic, comment.Type)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsConfidential)
}

func TestCountOpenIssuesForViewerConfidential(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	repo := a.repo

	// start from a clean slate: only the issues of this test count
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).Delete(&issues_model.Issue{})
	require.NoError(t, err)
	for range 3 {
		insertTestIssue(t, repo, a.other, false)
	}
	posterIssue := insertTestIssue(t, repo, a.reporter, true)
	insertTestIssue(t, repo, a.reader, true)
	closed := insertTestIssue(t, repo, a.reader, true)
	closed.IsClosed = true
	require.NoError(t, issues_model.UpdateIssueCols(ctx, closed, "is_closed"))
	repo.NumOpenIssues = 5 // 3 public + 2 open confidential, kept as the stored counter

	count := func(doer *user_model.User) (n int, statements int64) {
		scope, err := issues_model.BuildVisibilityScope(ctx, doer, []int64{repo.ID}, false)
		require.NoError(t, err)
		statements = countStatements(t, func() {
			n, err = issues_model.CountOpenIssuesForViewer(ctx, repo, scope)
			require.NoError(t, err)
		})
		return n, statements
	}

	n, _ := count(nil)
	assert.Equal(t, 3, n)
	n, _ = count(a.other)
	assert.Equal(t, 3, n)
	n, _ = count(a.reporter)
	assert.Equal(t, 4, n)
	n, statements := count(a.writer)
	assert.Equal(t, 5, n)
	assert.Zero(t, statements)
	n, _ = count(a.admin)
	assert.Equal(t, 5, n)
	assert.Equal(t, 5, repo.NumOpenIssues)
	assert.NotZero(t, posterIssue.ID)
}

func TestCountHiddenConfidentialIssuesByMilestone(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	milestone := &issues_model.Milestone{RepoID: a.repo.ID, Name: "confidential milestone", NumIssues: 8, NumClosedIssues: 4}
	require.NoError(t, db.Insert(ctx, milestone))
	for range 2 {
		issue := insertTestIssue(t, a.repo, a.reporter, true)
		issue.MilestoneID = milestone.ID
		require.NoError(t, issues_model.UpdateIssueCols(ctx, issue, "milestone_id"))
	}

	hiddenScope, err := issues_model.BuildVisibilityScope(ctx, a.other, []int64{a.repo.ID}, false)
	require.NoError(t, err)
	counts, err := issues_model.CountHiddenConfidentialIssuesByMilestone(ctx, hiddenScope, []int64{milestone.ID})
	require.NoError(t, err)
	assert.Equal(t, issues_model.HiddenIssueCounts{Open: 2}, counts[milestone.ID])

	hidden := &issues_model.Milestone{ID: milestone.ID, NumIssues: 8, NumClosedIssues: 4}
	require.NoError(t, issues_model.MilestoneList{hidden}.SubtractHiddenIssues(ctx, hiddenScope))
	assert.Equal(t, 6, hidden.NumIssues)
	assert.Equal(t, 4, hidden.NumClosedIssues)
	assert.Equal(t, 2, hidden.NumOpenIssues)
	assert.Equal(t, 66, hidden.Completeness)

	writerScope, err := issues_model.BuildVisibilityScope(ctx, a.writer, []int64{a.repo.ID}, false)
	require.NoError(t, err)
	visible := &issues_model.Milestone{ID: milestone.ID, NumIssues: 8, NumClosedIssues: 4}
	require.NoError(t, issues_model.MilestoneList{visible}.SubtractHiddenIssues(ctx, writerScope))
	assert.Equal(t, 4, visible.NumOpenIssues)
	assert.Equal(t, 50, visible.Completeness)

	stored := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: milestone.ID})
	assert.Equal(t, 8, stored.NumIssues)
	assert.Equal(t, 4, stored.NumClosedIssues)
}

func TestFindHiddenConfidentialIssues(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	cond := builder.Eq{"issue.repo_id": a.repo.ID}

	hidden, err := issues_model.FindHiddenConfidentialIssues(ctx, a.other, cond)
	require.NoError(t, err)
	ids := make([]int64, 0, len(hidden))
	for _, issue := range hidden {
		ids = append(ids, issue.ID)
	}
	assert.ElementsMatch(t, []int64{a.reporterIssue.ID, a.readerIssue.ID}, ids)

	hidden, err = issues_model.FindHiddenConfidentialIssues(ctx, a.admin, cond)
	require.NoError(t, err)
	assert.Nil(t, hidden)
}

func TestGetVisibleIssuesByIDs(t *testing.T) {
	a := setupConfidentialActors(t)
	ctx := t.Context()
	scope, err := issues_model.BuildVisibilityScope(ctx, a.other, []int64{a.repo.ID}, false)
	require.NoError(t, err)
	issues, err := issues_model.GetVisibleIssuesByIDs(ctx, []int64{a.publicIssue.ID, a.readerIssue.ID, a.reporterIssue.ID}, scope)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, a.publicIssue.ID, issues[0].ID)
}
