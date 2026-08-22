// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAuditNotifier(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()

	doer := &user_model.User{ID: 2, Name: "doer"}
	repo := &repo_model.Repository{ID: 1, OwnerName: "owner", Name: "repo"}
	ctx := context.WithValue(t.Context(), httplib.RequestContextKey, &http.Request{URL: &url.URL{Path: "/owner/repo"}})
	notifier := new(auditNotifier)

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
}
