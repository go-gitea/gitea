// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	packages_model "gitea.dev/models/packages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
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

	for _, tc := range []struct {
		notify func()
		event  audit_model.Event
	}{
		{
			func() {
				notifier.NewIssue(ctx, &issues_model.Issue{ID: 10, Index: 5, Title: "Issue title", Poster: doer, Repo: repo}, nil)
			},
			audit_model.Event{Action: audit_model.IssueCreate, Origin: audit_model.OriginUI},
		},
		{
			func() {
				notifier.NewPullRequest(ctx, &issues_model.PullRequest{ID: 11, Issue: &issues_model.Issue{ID: 12, Index: 6, Title: "PR title", Poster: doer, Repo: repo, IsPull: true}}, nil)
			},
			audit_model.Event{Action: audit_model.PullRequestCreate, Origin: audit_model.OriginUI},
		},
		{
			func() { notifier.NewWikiPage(ctx, doer, repo, "Home", "") },
			audit_model.Event{Action: audit_model.WikiPageCreate, Origin: audit_model.OriginUI},
		},
		{
			func() { notifier.CreateRepository(ctx, doer, doer, repo) },
			audit_model.Event{Action: audit_model.RepositoryCreate, ActorID: doer.ID, Origin: audit_model.OriginUI},
		},
		{
			func() { notifier.TransferRepository(ctx, doer, repo, &user_model.User{Name: "previous_owner"}) },
			audit_model.Event{Action: audit_model.RepositoryTransferFinish, ActorID: doer.ID, Message: "Transferred repository owner/repo from previous_owner to owner."},
		},
		{
			func() { notifier.ChangeDefaultBranch(ctx, repo) },
			audit_model.Event{Action: audit_model.RepositoryBranchDefault},
		},
		{
			func() {
				notifier.PullReviewDismiss(ctx, doer, &issues_model.Review{ID: 41, Issue: &issues_model.Issue{Index: 8, Repo: repo}}, nil)
			},
			audit_model.Event{Action: audit_model.PullRequestReviewDismiss, ActorID: doer.ID, Message: "Dismissed review 41 of pull request #8 in repository owner/repo."},
		},
		{
			func() { notifier.DeleteRef(ctx, doer, repo, git.RefNameFromTag("v1.0")) },
			audit_model.Event{Action: audit_model.RepositoryTagRemove, ActorID: doer.ID, Message: "Removed tag v1.0 from repository owner/repo."},
		},
	} {
		tc.notify()
		tc.event.ScopeType = audit_model.ScopeRepository
		tc.event.ScopeID = repo.ID
		unittest.AssertExistsAndLoadBean(t, &tc.event)
	}

	notifier.DeleteRef(ctx, doer, repo, git.RefNameFromBranch("feature"))
	unittest.AssertCount(t, &audit_model.Event{Action: audit_model.RepositoryTagRemove}, 1)

	org := &user_model.User{ID: 3, Name: "org3", Type: user_model.UserTypeOrganization}
	impersonatedCtx := WithImpersonator(ctx, &user_model.User{ID: 1, Name: "admin"})
	notifier.PackageDelete(impersonatedCtx, doer, &packages_model.PackageDescriptor{Owner: org, Package: &packages_model.Package{Name: "pkg"}, Version: &packages_model.PackageVersion{Version: "1.0.0"}})
	unittest.AssertExistsAndLoadBean(t, &audit_model.Event{Action: audit_model.OrganizationPackageRemove, ActorID: doer.ID, ImpersonatorID: 1, ScopeType: audit_model.ScopeOrganization, ScopeID: org.ID})
}
