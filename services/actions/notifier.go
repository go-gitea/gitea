// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"
	notify_service "code.gitea.io/gitea/services/notify"
)

type actionsNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &actionsNotifier{}

// NewNotifier create a new actionsNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &actionsNotifier{}
}

// NewIssue notifies issue created event
func (n *actionsNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, _ []*user_model.User) {
	ctx = withMethod(ctx, "NewIssue")
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)

	newNotifyInputFromIssue(issue, webhook_module.HookEventIssues).WithPayload(&api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, issue.Poster, issue),
		Repository: convert.ToRepo(ctx, issue.Repo, permission),
		Sender:     convert.ToUser(ctx, issue.Poster, nil),
	}).Notify(withMethod(ctx, "NewIssue"))
}

// IssueChangeContent notifies change content of issue
func (n *actionsNotifier) IssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	ctx = withMethod(ctx, "IssueChangeContent")
	n.notifyIssueChangeWithTitleOrContent(ctx, doer, issue)
}

func (n *actionsNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	ctx = withMethod(ctx, "IssueChangeTitle")
	n.notifyIssueChangeWithTitleOrContent(ctx, doer, issue)
}

func (n *actionsNotifier) notifyIssueChangeWithTitleOrContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	var err error
	if err = issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		newNotifyInputFromIssue(issue, webhook_module.HookEventPullRequest).
			WithDoer(doer).
			WithPayload(&api.PullRequestPayload{
				Action:      api.HookIssueEdited,
				Index:       issue.Index,
				PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
				Repository:  convert.ToRepo(ctx, issue.Repo, access_model.Permission{AccessMode: perm_model.AccessModeNone}),
				Sender:      convert.ToUser(ctx, doer, nil),
			}).
			WithPullRequest(issue.PullRequest).
			Notify(ctx)
		return
	}
	newNotifyInputFromIssue(issue, webhook_module.HookEventIssues).
		WithDoer(doer).
		WithPayload(&api.IssuePayload{
			Action:     api.HookIssueEdited,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		}).
		Notify(ctx)
}

// IssueChangeStatus notifies close or reopen issue to notifiers
func (n *actionsNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	ctx = withMethod(ctx, "IssueChangeStatus")
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(ctx, issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
			CommitID:    commitID,
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		newNotifyInputFromIssue(issue, webhook_module.HookEventPullRequest).
			WithDoer(doer).
			WithPayload(apiPullRequest).
			WithPullRequest(issue.PullRequest).
			Notify(ctx)
		return
	}
	apiIssue := &api.IssuePayload{
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(ctx, doer, issue),
		Repository: convert.ToRepo(ctx, issue.Repo, permission),
		Sender:     convert.ToUser(ctx, doer, nil),
	}
	if isClosed {
		apiIssue.Action = api.HookIssueClosed
	} else {
		apiIssue.Action = api.HookIssueReOpened
	}
	newNotifyInputFromIssue(issue, webhook_module.HookEventIssues).
		WithDoer(doer).
		WithPayload(apiIssue).
		Notify(ctx)
}

// IssueChangeAssignee notifies assigned or unassigned to notifiers
func (n *actionsNotifier) IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	ctx = withMethod(ctx, "IssueChangeAssignee")

	var action api.HookIssueAction
	if removed {
		action = api.HookIssueUnassigned
	} else {
		action = api.HookIssueAssigned
	}

	hookEvent := webhook_module.HookEventIssueAssign
	if issue.IsPull {
		hookEvent = webhook_module.HookEventPullRequestAssign
	}

	notifyIssueChange(ctx, doer, issue, hookEvent, action)
}

// IssueChangeMilestone notifies assignee to notifiers
func (n *actionsNotifier) IssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	ctx = withMethod(ctx, "IssueChangeMilestone")

	var action api.HookIssueAction
	if issue.MilestoneID > 0 {
		action = api.HookIssueMilestoned
	} else {
		action = api.HookIssueDemilestoned
	}

	hookEvent := webhook_module.HookEventIssueMilestone
	if issue.IsPull {
		hookEvent = webhook_module.HookEventPullRequestMilestone
	}

	notifyIssueChange(ctx, doer, issue, hookEvent, action)
}

func (n *actionsNotifier) IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	_, _ []*issues_model.Label,
) {
	ctx = withMethod(ctx, "IssueChangeLabels")

	hookEvent := webhook_module.HookEventIssueLabel
	if issue.IsPull {
		hookEvent = webhook_module.HookEventPullRequestLabel
	}

	notifyIssueChange(ctx, doer, issue, hookEvent, api.HookIssueLabelUpdated)
}

func notifyIssueChange(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, event webhook_module.HookEventType, action api.HookIssueAction) {
	var err error
	if err = issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		newNotifyInputFromIssue(issue, event).
			WithDoer(doer).
			WithPayload(&api.PullRequestPayload{
				Action:      action,
				Index:       issue.Index,
				PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
				Repository:  convert.ToRepo(ctx, issue.Repo, access_model.Permission{AccessMode: perm_model.AccessModeNone}),
				Sender:      convert.ToUser(ctx, doer, nil),
			}).
			WithPullRequest(issue.PullRequest).
			Notify(ctx)
		return
	}
	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, issue.Poster)
	newNotifyInputFromIssue(issue, event).
		WithDoer(doer).
		WithPayload(&api.IssuePayload{
			Action:     action,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, doer, issue),
			Repository: convert.ToRepo(ctx, issue.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		}).
		Notify(ctx)
}

// CreateIssueComment notifies comment on an issue to notifiers
func (n *actionsNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, _ []*user_model.User,
) {
	ctx = withMethod(ctx, "CreateIssueComment")

	if issue.IsPull {
		notifyIssueCommentChange(ctx, doer, comment, "", webhook_module.HookEventPullRequestComment, api.HookIssueCommentCreated)
		return
	}
	notifyIssueCommentChange(ctx, doer, comment, "", webhook_module.HookEventIssueComment, api.HookIssueCommentCreated)
}

func (n *actionsNotifier) UpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
	ctx = withMethod(ctx, "UpdateComment")

	if err := c.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if c.Issue.IsPull {
		notifyIssueCommentChange(ctx, doer, c, oldContent, webhook_module.HookEventPullRequestComment, api.HookIssueCommentEdited)
		return
	}
	notifyIssueCommentChange(ctx, doer, c, oldContent, webhook_module.HookEventIssueComment, api.HookIssueCommentEdited)
}

func (n *actionsNotifier) DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) {
	ctx = withMethod(ctx, "DeleteComment")

	if err := comment.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if comment.Issue.IsPull {
		notifyIssueCommentChange(ctx, doer, comment, "", webhook_module.HookEventPullRequestComment, api.HookIssueCommentDeleted)
		return
	}
	notifyIssueCommentChange(ctx, doer, comment, "", webhook_module.HookEventIssueComment, api.HookIssueCommentDeleted)
}

func notifyIssueCommentChange(ctx context.Context, doer *user_model.User, comment *issues_model.Comment, oldContent string, event webhook_module.HookEventType, action api.HookIssueCommentAction) {
	if err := comment.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := comment.Issue.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, comment.Issue.Repo, doer)

	payload := &api.IssueCommentPayload{
		Action:     action,
		Issue:      convert.ToAPIIssue(ctx, doer, comment.Issue),
		Comment:    convert.ToAPIComment(ctx, comment.Issue.Repo, comment),
		Repository: convert.ToRepo(ctx, comment.Issue.Repo, permission),
		Sender:     convert.ToUser(ctx, doer, nil),
		IsPull:     comment.Issue.IsPull,
	}

	if action == api.HookIssueCommentEdited {
		payload.Changes = &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		}
	}

	if comment.Issue.IsPull {
		if err := comment.Issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		newNotifyInputFromIssue(comment.Issue, event).
			WithDoer(doer).
			WithPayload(payload).
			WithPullRequest(comment.Issue.PullRequest).
			Notify(ctx)
		return
	}

	newNotifyInputFromIssue(comment.Issue, event).
		WithDoer(doer).
		WithPayload(payload).
		Notify(ctx)
}

func (n *actionsNotifier) NewPullRequest(ctx context.Context, pull *issues_model.PullRequest, _ []*user_model.User) {
	ctx = withMethod(ctx, "NewPullRequest")

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

	permission, _ := access_model.GetUserRepoPermission(ctx, pull.Issue.Repo, pull.Issue.Poster)

	newNotifyInputFromIssue(pull.Issue, webhook_module.HookEventPullRequest).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueOpened,
			Index:       pull.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, pull, nil),
			Repository:  convert.ToRepo(ctx, pull.Issue.Repo, permission),
			Sender:      convert.ToUser(ctx, pull.Issue.Poster, nil),
		}).
		WithPullRequest(pull).
		Notify(ctx)
}

func (n *actionsNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	ctx = withMethod(ctx, "CreateRepository")

	newNotifyInput(repo, doer, webhook_module.HookEventRepository).WithPayload(&api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
		Organization: convert.ToUser(ctx, u, nil),
		Sender:       convert.ToUser(ctx, doer, nil),
	}).Notify(ctx)
}

func (n *actionsNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	ctx = withMethod(ctx, "ForkRepository")

	oldPermission, _ := access_model.GetUserRepoPermission(ctx, oldRepo, doer)
	permission, _ := access_model.GetUserRepoPermission(ctx, repo, doer)

	// forked webhook
	newNotifyInput(oldRepo, doer, webhook_module.HookEventFork).WithPayload(&api.ForkPayload{
		Forkee: convert.ToRepo(ctx, oldRepo, oldPermission),
		Repo:   convert.ToRepo(ctx, repo, permission),
		Sender: convert.ToUser(ctx, doer, nil),
	}).Notify(ctx)

	u := repo.MustOwner(ctx)

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		newNotifyInput(repo, doer, webhook_module.HookEventRepository).
			WithRef(git.RefNameFromBranch(oldRepo.DefaultBranch).String()).
			WithPayload(&api.RepositoryPayload{
				Action:       api.HookRepoCreated,
				Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
				Organization: convert.ToUser(ctx, u, nil),
				Sender:       convert.ToUser(ctx, doer, nil),
			}).Notify(ctx)
	}
}

func (n *actionsNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, _ *issues_model.Comment, _ []*user_model.User) {
	ctx = withMethod(ctx, "PullRequestReview")

	var reviewHookType webhook_module.HookEventType

	switch review.Type {
	case issues_model.ReviewTypeApprove:
		reviewHookType = webhook_module.HookEventPullRequestReviewApproved
	case issues_model.ReviewTypeComment:
		reviewHookType = webhook_module.HookEventPullRequestReviewComment
	case issues_model.ReviewTypeReject:
		reviewHookType = webhook_module.HookEventPullRequestReviewRejected
	default:
		// unsupported review webhook type here
		log.Error("Unsupported review webhook type")
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}

	permission, err := access_model.GetUserRepoPermission(ctx, review.Issue.Repo, review.Issue.Poster)
	if err != nil {
		log.Error("models.GetUserRepoPermission: %v", err)
		return
	}

	newNotifyInput(review.Issue.Repo, review.Reviewer, reviewHookType).
		WithRef(review.CommitID).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueReviewed,
			Index:       review.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
			Repository:  convert.ToRepo(ctx, review.Issue.Repo, permission),
			Sender:      convert.ToUser(ctx, review.Reviewer, nil),
			Review: &api.ReviewPayload{
				Type:    string(reviewHookType),
				Content: review.Content,
			},
		}).Notify(ctx)
}

func (n *actionsNotifier) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if !issue.IsPull {
		log.Warn("PullRequestReviewRequest: issue is not a pull request: %v", issue.ID)
		return
	}

	ctx = withMethod(ctx, "PullRequestReviewRequest")

	permission, _ := access_model.GetUserRepoPermission(ctx, issue.Repo, doer)
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest failed: %v", err)
		return
	}
	var action api.HookIssueAction
	if isRequest {
		action = api.HookIssueReviewRequested
	} else {
		action = api.HookIssueReviewRequestRemoved
	}
	newNotifyInputFromIssue(issue, webhook_module.HookEventPullRequestReviewRequest).
		WithDoer(doer).
		WithPayload(&api.PullRequestPayload{
			Action:            action,
			Index:             issue.Index,
			PullRequest:       convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			RequestedReviewer: convert.ToUser(ctx, reviewer, nil),
			Repository:        convert.ToRepo(ctx, issue.Repo, permission),
			Sender:            convert.ToUser(ctx, doer, nil),
		}).
		WithPullRequest(issue.PullRequest).
		Notify(ctx)
}

func (*actionsNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "MergePullRequest")

	// Reload pull request information.
	if err := pr.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	permission, err := access_model.GetUserRepoPermission(ctx, pr.Issue.Repo, doer)
	if err != nil {
		log.Error("models.GetUserRepoPermission: %v", err)
		return
	}

	// Merge pull request calls issue.changeStatus so we need to handle separately.
	apiPullRequest := &api.PullRequestPayload{
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
		Repository:  convert.ToRepo(ctx, pr.Issue.Repo, permission),
		Sender:      convert.ToUser(ctx, doer, nil),
		Action:      api.HookIssueClosed,
	}

	newNotifyInput(pr.Issue.Repo, doer, webhook_module.HookEventPullRequest).
		WithRef(pr.MergedCommitID).
		WithPayload(apiPullRequest).
		WithPullRequest(pr).
		Notify(ctx)
}

func (n *actionsNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	commitID, _ := git.NewIDFromString(opts.NewCommitID)
	if commitID.IsZero() {
		log.Trace("new commitID is empty")
		return
	}

	ctx = withMethod(ctx, "PushCommits")

	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	newNotifyInput(repo, pusher, webhook_module.HookEventPush).
		WithRef(opts.RefFullName.String()).
		WithPayload(&api.PushPayload{
			Ref:        opts.RefFullName.String(),
			Before:     opts.OldCommitID,
			After:      opts.NewCommitID,
			CompareURL: setting.AppURL + commits.CompareURL,
			Commits:    apiCommits,
			HeadCommit: apiHeadCommit,
			Repo:       convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
			Pusher:     apiPusher,
			Sender:     apiPusher,
		}).
		Notify(ctx)
}

func (n *actionsNotifier) CreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	ctx = withMethod(ctx, "CreateRef")

	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiRepo := convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeNone})

	newNotifyInput(repo, pusher, webhook_module.HookEventCreate).
		WithRef(refFullName.String()).
		WithPayload(&api.CreatePayload{
			Ref:     refFullName.String(), // HINT: here is inconsistent with the Webhook's payload: webhook uses ShortName
			Sha:     refID,
			RefType: string(refFullName.RefType()),
			Repo:    apiRepo,
			Sender:  apiPusher,
		}).
		Notify(ctx)
}

func (n *actionsNotifier) DeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	ctx = withMethod(ctx, "DeleteRef")

	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiRepo := convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeNone})

	newNotifyInput(repo, pusher, webhook_module.HookEventDelete).
		WithPayload(&api.DeletePayload{
			Ref:        refFullName.String(), // HINT: here is inconsistent with the Webhook's payload: webhook uses ShortName
			RefType:    string(refFullName.RefType()),
			PusherType: api.PusherTypeUser,
			Repo:       apiRepo,
			Sender:     apiPusher,
		}).
		Notify(ctx)
}

func (n *actionsNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx = withMethod(ctx, "SyncPushCommits")

	apiPusher := convert.ToUser(ctx, pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	newNotifyInput(repo, pusher, webhook_module.HookEventPush).
		WithRef(opts.RefFullName.String()).
		WithPayload(&api.PushPayload{
			Ref:          opts.RefFullName.String(),
			Before:       opts.OldCommitID,
			After:        opts.NewCommitID,
			CompareURL:   setting.AppURL + commits.CompareURL,
			Commits:      apiCommits,
			TotalCommits: commits.Len,
			HeadCommit:   apiHeadCommit,
			Repo:         convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
			Pusher:       apiPusher,
			Sender:       apiPusher,
		}).
		Notify(ctx)
}

func (n *actionsNotifier) SyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	ctx = withMethod(ctx, "SyncCreateRef")
	n.CreateRef(ctx, pusher, repo, refFullName, refID)
}

func (n *actionsNotifier) SyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	ctx = withMethod(ctx, "SyncDeleteRef")
	n.DeleteRef(ctx, pusher, repo, refFullName)
}

func (n *actionsNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	ctx = withMethod(ctx, "NewRelease")
	notifyRelease(ctx, rel.Publisher, rel, api.HookReleasePublished)
}

func (n *actionsNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	ctx = withMethod(ctx, "UpdateRelease")
	notifyRelease(ctx, doer, rel, api.HookReleaseUpdated)
}

func (n *actionsNotifier) DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel.IsTag {
		// has sent same action in `PushCommits`, so skip it.
		return
	}
	ctx = withMethod(ctx, "DeleteRelease")
	notifyRelease(ctx, doer, rel, api.HookReleaseDeleted)
}

func (n *actionsNotifier) PackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	ctx = withMethod(ctx, "PackageCreate")
	notifyPackage(ctx, doer, pd, api.HookPackageCreated)
}

func (n *actionsNotifier) PackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	ctx = withMethod(ctx, "PackageDelete")
	notifyPackage(ctx, doer, pd, api.HookPackageDeleted)
}

func (n *actionsNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "AutoMergePullRequest")
	n.MergePullRequest(ctx, doer, pr)
}

func (n *actionsNotifier) PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ctx = withMethod(ctx, "PullRequestSynchronized")

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	newNotifyInput(pr.Issue.Repo, doer, webhook_module.HookEventPullRequestSync).
		WithPayload(&api.PullRequestPayload{
			Action:      api.HookIssueSynchronized,
			Index:       pr.Issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
			Repository:  convert.ToRepo(ctx, pr.Issue.Repo, access_model.Permission{AccessMode: perm_model.AccessModeNone}),
			Sender:      convert.ToUser(ctx, doer, nil),
		}).
		WithPullRequest(pr).
		Notify(ctx)
}

func (n *actionsNotifier) PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	ctx = withMethod(ctx, "PullRequestChangeTargetBranch")

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, pr.Issue.Repo, pr.Issue.Poster)
	newNotifyInput(pr.Issue.Repo, doer, webhook_module.HookEventPullRequest).
		WithPayload(&api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  pr.Issue.Index,
			Changes: &api.ChangesPayload{
				Ref: &api.ChangesFromPayload{
					From: oldBranch,
				},
			},
			PullRequest: convert.ToAPIPullRequest(ctx, pr, nil),
			Repository:  convert.ToRepo(ctx, pr.Issue.Repo, permission),
			Sender:      convert.ToUser(ctx, doer, nil),
		}).
		WithPullRequest(pr).
		Notify(ctx)
}

func (n *actionsNotifier) NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	ctx = withMethod(ctx, "NewWikiPage")

	newNotifyInput(repo, doer, webhook_module.HookEventWiki).WithPayload(&api.WikiPayload{
		Action:     api.HookWikiCreated,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
		Comment:    comment,
	}).Notify(ctx)
}

func (n *actionsNotifier) EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	ctx = withMethod(ctx, "EditWikiPage")

	newNotifyInput(repo, doer, webhook_module.HookEventWiki).WithPayload(&api.WikiPayload{
		Action:     api.HookWikiEdited,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
		Comment:    comment,
	}).Notify(ctx)
}

func (n *actionsNotifier) DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	ctx = withMethod(ctx, "DeleteWikiPage")

	newNotifyInput(repo, doer, webhook_module.HookEventWiki).WithPayload(&api.WikiPayload{
		Action:     api.HookWikiDeleted,
		Repository: convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
		Sender:     convert.ToUser(ctx, doer, nil),
		Page:       page,
	}).Notify(ctx)
}

// MigrateRepository is used to detect workflows after a repository has been migrated
func (n *actionsNotifier) MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	ctx = withMethod(ctx, "MigrateRepository")

	newNotifyInput(repo, doer, webhook_module.HookEventRepository).WithPayload(&api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(ctx, repo, access_model.Permission{AccessMode: perm_model.AccessModeOwner}),
		Organization: convert.ToUser(ctx, u, nil),
		Sender:       convert.ToUser(ctx, doer, nil),
	}).Notify(ctx)
}
