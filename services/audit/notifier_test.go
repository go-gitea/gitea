// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAuditNotifier(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	doer := &user_model.User{ID: 2, Name: "doer"}
	repo := &repo_model.Repository{ID: 1, OwnerName: "owner", Name: "repo"}
	ctx := reqctx.NewRequestContextForTest(t)
	SetRequestInfo(ctx, audit_model.OriginUI, "127.0.0.1")
	notifier := new(auditNotifier)
	migrator := &user_model.User{ID: 3, Name: "migrator"}

	issue := &issues_model.Issue{ID: 10, Index: 5, Title: "Issue title", Poster: doer, Repo: repo}
	notifier.NewIssue(ctx, issue, nil)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.IssueCreate,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Origin:    audit_model.OriginUI,
	})

	pr := &issues_model.PullRequest{ID: 11, Issue: &issues_model.Issue{ID: 12, Index: 6, Title: "PR title", Poster: doer, Repo: repo, IsPull: true}}
	notifier.NewPullRequest(ctx, pr, nil)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.PullRequestCreate,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Origin:    audit_model.OriginUI,
	})

	notifier.NewWikiPage(ctx, doer, repo, "Home", "")
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.WikiPageCreate,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Origin:    audit_model.OriginUI,
	})

	notifier.CreateRepository(ctx, doer, doer, repo)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.RepositoryCreate,
		ActorID:   doer.ID,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Origin:    audit_model.OriginUI,
	})

	notifier.TransferRepository(ctx, doer, repo, "previous_owner")
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.RepositoryTransferFinish,
		ActorID:   doer.ID,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Message:   "Transferred repository owner/repo from previous_owner to owner.",
	})

	notifier.ChangeDefaultBranch(ctx, repo)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.RepositoryBranchDefault,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
	})

	notifier.MigrateRepository(ctx, migrator, migrator, repo)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.RepositoryCreate,
		ActorID:   migrator.ID,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Origin:    audit_model.OriginUI,
	})

	autoMergePR := &issues_model.PullRequest{Issue: &issues_model.Issue{Index: 7, Repo: repo}}
	notifier.AutoMergePullRequest(ctx, doer, autoMergePR)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.PullRequestMerge,
		ActorID:   doer.ID,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
	})

	notifier.PullRequestCodeComment(ctx, autoMergePR, &issues_model.Comment{ID: 32, Poster: doer}, nil)
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action:    audit_model.PullRequestCommentCreate,
		ActorID:   doer.ID,
		ScopeType: audit_model.ScopeRepository,
		ScopeID:   repo.ID,
		Message:   "Added comment 32 to pull request #7 in repository owner/repo.",
	})

	notifier.PullRequestReview(ctx, autoMergePR, &issues_model.Review{ID: 21, Type: issues_model.ReviewTypeApprove, Reviewer: doer}, &issues_model.Comment{ID: 31}, nil)
	assert.JSONEq(t, `{"pull_request":"#7","comment_id":31,"review_id":21,"review_type":"approve"}`, unittest.AssertExistsAndLoadBean(t, &audit_model.Event{
		Action: audit_model.PullRequestCommentCreate, ActorID: doer.ID, Message: "Added comment 31 to pull request #7 in repository owner/repo.",
	}).Metadata)
}
