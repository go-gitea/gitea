// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

type botsNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &botsNotifier{}

// NewNotifier create a new botsNotifier notifier
func NewNotifier() base.Notifier {
	return &botsNotifier{}
}

// NotifyNewIssue notifies issue created event
func (n *botsNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, _ []*user_model.User) {
	ctx = withMethod(ctx, "NotifyNewIssue")
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)

	newNotifyInputFromIssue(issue, webhook.HookEventIssues).WithPayload(&api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, issue),
		Repository: convert.ToRepo(issue.Repo, mode),
		Sender:     convert.ToUser(issue.Poster, nil),
	}).Notify(withMethod(ctx, "NotifyNewIssue"))
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (n *botsNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	ctx = withMethod(ctx, "NotifyIssueChangeStatus")
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(db.DefaultContext, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		newNotifyInputFromIssue(issue, webhook.HookEventPullRequest).
			WithDoer(doer).
			WithPayload(apiPullRequest).
			Notify(ctx)
		return
	}
	apiIssue := &api.IssuePayload{
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, issue),
		Repository: convert.ToRepo(issue.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
	}
	if isClosed {
		apiIssue.Action = api.HookIssueClosed
	} else {
		apiIssue.Action = api.HookIssueReOpened
	}
	newNotifyInputFromIssue(issue, webhook.HookEventIssues).
		WithDoer(doer).
		WithPayload(apiIssue).
		Notify(ctx)
}

func (n *botsNotifier) NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	_, _ []*issues_model.Label,
) {
	ctx = withMethod(ctx, "NotifyIssueChangeLabels")

	var err error
	if err = issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(ctx); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		newNotifyInputFromIssue(issue, webhook.HookEventPullRequestLabel).
			WithDoer(doer).
			WithPayload(&api.PullRequestPayload{
				Action:      api.HookIssueLabelUpdated,
				Index:       issue.Index,
				PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
				Repository:  convert.ToRepo(issue.Repo, perm.AccessModeNone),
				Sender:      convert.ToUser(doer, nil),
			}).
			Notify(ctx)
		return
	}
	newNotifyInputFromIssue(issue, webhook.HookEventIssueLabel).
		WithDoer(doer).
		WithPayload(&api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}).
		Notify(ctx)
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (n *botsNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, _ []*user_model.User,
) {
	ctx = withMethod(ctx, "NotifyCreateIssueComment")

	mode, _ := access_model.AccessLevel(ctx, doer, repo)

	if issue.IsPull {
		newNotifyInputFromIssue(issue, webhook.HookEventPullRequestComment).
			WithDoer(doer).
			WithPayload(&api.IssueCommentPayload{
				Action:     api.HookIssueCommentCreated,
				Issue:      convert.ToAPIIssue(ctx, issue),
				Comment:    convert.ToComment(comment),
				Repository: convert.ToRepo(repo, mode),
				Sender:     convert.ToUser(doer, nil),
				IsPull:     true,
			}).
			Notify(ctx)
		return
	}
	newNotifyInputFromIssue(issue, webhook.HookEventIssueComment).
		WithDoer(doer).
		WithPayload(&api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		}).
		Notify(ctx)
}

func (n *botsNotifier) NotifyNewPullRequest(ctx context.Context, pull *issues_model.PullRequest, _ []*user_model.User) {
	ctx = withMethod(ctx, "NotifyNewPullRequest")

	if err := pull.LoadIssue(ctx); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(ctx); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(ctx); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, pull.Issue.Poster, pull.Issue.Repo)

	newNotifyInputFromIssue(pull.Issue, webhook.HookEventPullRequest).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueOpened,
			Index:       pull.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, pull, nil),
			Repository:  convert.ToRepo(pull.Issue.Repo, mode),
			Sender:      convert.ToUser(pull.Issue.Poster, nil),
		}).
		WithPullRequest(pull).
		Notify(ctx)
}

func (n *botsNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	ctx = withMethod(ctx, "NotifyCreateRepository")

	newNotifyInput(repo, doer, webhook.HookEventRepository).WithPayload(&api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}).Notify(ctx)
}

func (n *botsNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	ctx = withMethod(ctx, "NotifyForkRepository")

	oldMode, _ := access_model.AccessLevel(ctx, doer, oldRepo)
	mode, _ := access_model.AccessLevel(ctx, doer, repo)

	// forked webhook
	newNotifyInput(oldRepo, doer, webhook.HookEventFork).WithPayload(&api.ForkPayload{
		Forkee: convert.ToRepo(oldRepo, oldMode),
		Repo:   convert.ToRepo(repo, mode),
		Sender: convert.ToUser(doer, nil),
	}).Notify(ctx)

	u := repo.MustOwner(ctx)

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		newNotifyInput(repo, doer, webhook.HookEventRepository).
			WithRef(oldRepo.DefaultBranch).
			WithPayload(&api.RepositoryPayload{
				Action:       api.HookRepoCreated,
				Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
				Organization: convert.ToUser(u, nil),
				Sender:       convert.ToUser(doer, nil),
			}).Notify(ctx)
	}
}

func (n *botsNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, _ *issues_model.Comment, _ []*user_model.User) {
	ctx = withMethod(ctx, "NotifyPullRequestReview")

	var reviewHookType webhook.HookEventType

	switch review.Type {
	case issues_model.ReviewTypeApprove:
		reviewHookType = webhook.HookEventPullRequestReviewApproved
	case issues_model.ReviewTypeComment:
		reviewHookType = webhook.HookEventPullRequestComment
	case issues_model.ReviewTypeReject:
		reviewHookType = webhook.HookEventPullRequestReviewRejected
	default:
		// unsupported review webhook type here
		log.Error("Unsupported review webhook type")
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(ctx, review.Issue.Poster, review.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}

	newNotifyInput(review.Issue.Repo, review.Reviewer, reviewHookType).
		WithRef(review.CommitID).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueReviewed,
			Index:       review.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(db.DefaultContext, pr, nil),
			Repository:  convert.ToRepo(review.Issue.Repo, mode),
			Sender:      convert.ToUser(review.Reviewer, nil),
			Review: &api.ReviewPayload{
				Type:    string(reviewHookType),
				Content: review.Content,
			},
		}).Notify(ctx)
}

func (*botsNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "NotifyMergePullRequest")

	// Reload pull request information.
	if err := pr.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(db.DefaultContext); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	mode, err := access_model.AccessLevel(ctx, doer, pr.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}

	// Merge pull request calls issue.changeStatus so we need to handle separately.
	apiPullRequest := &api.PullRequestPayload{
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(db.DefaultContext, pr, nil),
		Repository:  convert.ToRepo(pr.Issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
		Action:      api.HookIssueClosed,
	}

	newNotifyInput(pr.Issue.Repo, doer, webhook.HookEventPullRequest).
		WithRef(pr.MergedCommitID).
		WithPayload(apiPullRequest).
		WithPullRequest(pr).
		Notify(ctx)
}

func (n *botsNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx = withMethod(ctx, "NotifyPushCommits")

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	newNotifyInput(repo, pusher, webhook.HookEventPush).
		WithRef(opts.RefFullName).
		WithPayload(&api.PushPayload{
			Ref:        opts.RefFullName,
			Before:     opts.OldCommitID,
			After:      opts.NewCommitID,
			CompareURL: setting.AppURL + commits.CompareURL,
			Commits:    apiCommits,
			HeadCommit: apiHeadCommit,
			Repo:       convert.ToRepo(repo, perm.AccessModeOwner),
			Pusher:     apiPusher,
			Sender:     apiPusher,
		}).
		Notify(ctx)
}

func (n *botsNotifier) NotifyCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	ctx = withMethod(ctx, "NotifyCreateRef")

	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	newNotifyInput(repo, pusher, webhook.HookEventCreate).
		WithRef(refName).
		WithPayload(&api.CreatePayload{
			Ref:     refName,
			Sha:     refID,
			RefType: refType,
			Repo:    apiRepo,
			Sender:  apiPusher,
		}).
		Notify(ctx)
}

func (n *botsNotifier) NotifyDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	ctx = withMethod(ctx, "NotifyDeleteRef")

	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	newNotifyInput(repo, pusher, webhook.HookEventDelete).
		WithRef(refName).
		WithPayload(&api.DeletePayload{
			Ref:        refName,
			RefType:    refType,
			PusherType: api.PusherTypeUser,
			Repo:       apiRepo,
			Sender:     apiPusher,
		}).
		Notify(ctx)
}

func (n *botsNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx = withMethod(ctx, "NotifySyncPushCommits")

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(db.DefaultContext, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	newNotifyInput(repo, pusher, webhook.HookEventPush).
		WithRef(opts.RefFullName).
		WithPayload(&api.PushPayload{
			Ref:          opts.RefFullName,
			Before:       opts.OldCommitID,
			After:        opts.NewCommitID,
			CompareURL:   setting.AppURL + commits.CompareURL,
			Commits:      apiCommits,
			TotalCommits: commits.Len,
			HeadCommit:   apiHeadCommit,
			Repo:         convert.ToRepo(repo, perm.AccessModeOwner),
			Pusher:       apiPusher,
			Sender:       apiPusher,
		}).
		Notify(ctx)
}

func (n *botsNotifier) NotifySyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	ctx = withMethod(ctx, "NotifySyncCreateRef")
	n.NotifyCreateRef(ctx, pusher, repo, refType, refFullName, refID)
}

func (n *botsNotifier) NotifySyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	ctx = withMethod(ctx, "NotifySyncDeleteRef")
	n.NotifyDeleteRef(ctx, pusher, repo, refType, refFullName)
}

func (n *botsNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	ctx = withMethod(ctx, "NotifyNewRelease")
	notifyRelease(ctx, rel.Publisher, rel, rel.Sha1, api.HookReleasePublished)
}

func (n *botsNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	ctx = withMethod(ctx, "NotifyUpdateRelease")
	notifyRelease(ctx, doer, rel, rel.Sha1, api.HookReleaseUpdated)
}

func (n *botsNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	ctx = withMethod(ctx, "NotifyDeleteRelease")
	notifyRelease(ctx, doer, rel, rel.Sha1, api.HookReleaseDeleted)
}

func (n *botsNotifier) NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	ctx = withMethod(ctx, "NotifyPackageCreate")
	notifyPackage(ctx, doer, pd, api.HookPackageCreated)
}

func (n *botsNotifier) NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	ctx = withMethod(ctx, "NotifyPackageDelete")
	notifyPackage(ctx, doer, pd, api.HookPackageDeleted)
}

func (n *botsNotifier) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "NotifyAutoMergePullRequest")
	n.NotifyMergePullRequest(ctx, doer, pr)
}

func (n *botsNotifier) NotifyPullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "NotifyPullRequestSynchronized")

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(db.DefaultContext); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	newNotifyInput(pr.Issue.Repo, doer, webhook.HookEventPullRequestSync).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueSynchronized,
			Index:       pr.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
			Repository:  convert.ToRepo(pr.Issue.Repo, perm.AccessModeNone),
			Sender:      convert.ToUser(doer, nil),
		}).
		WithPullRequest(pr).
		Notify(ctx)
}

func (n *botsNotifier) NotifyPullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	ctx = withMethod(ctx, "NotifyPullRequestChangeTargetBranch")

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(db.DefaultContext); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, pr.Issue.Poster, pr.Issue.Repo)
	newNotifyInput(pr.Issue.Repo, doer, webhook.HookEventPullRequest).
		WithPayload(&api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  pr.Issue.Index,
			Changes: &api.ChangesPayload{
				Ref: &api.ChangesFromPayload{
					From: oldBranch,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
			Repository:  convert.ToRepo(pr.Issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}).
		WithPullRequest(pr).
		Notify(ctx)
}
