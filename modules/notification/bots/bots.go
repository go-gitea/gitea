// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	bots_module "code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/nektos/act/pkg/jobparser"
)

type botsNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &botsNotifier{}

// NewNotifier create a new botsNotifier notifier
func NewNotifier() base.Notifier {
	return &botsNotifier{}
}

func notify(repo *repo_model.Repository, doer *user_model.User, ref string, evt webhook.HookEventType, payload api.Payloader) error {
	return notifyWithPR(repo, doer, ref, evt, payload, nil)
}

func notifyWithPR(repo *repo_model.Repository, doer *user_model.User, ref string, evt webhook.HookEventType, payload api.Payloader, pr *issues_model.PullRequest) error {
	if unit.TypeBots.UnitGlobalDisabled() {
		return nil
	}
	if err := repo.LoadUnits(db.DefaultContext); err != nil {
		return fmt.Errorf("repo.LoadUnits: %w", err)
	} else if !repo.UnitEnabled(unit.TypeBots) {
		return nil
	}

	gitRepo, err := git.OpenRepository(context.Background(), repo.RepoPath())
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %v", err)
	}

	workflows, err := bots_module.DetectWorkflows(commit, evt)
	if err != nil {
		return fmt.Errorf("DetectWorkflows: %v", err)
	}

	if len(workflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", repo.RepoPath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %v", err)
	}

	for id, content := range workflows {
		run := bots_model.Run{
			Title:             commit.Message(),
			RepoID:            repo.ID,
			OwnerID:           repo.OwnerID,
			WorkflowID:        id,
			TriggerUserID:     doer.ID,
			Ref:               ref,
			CommitSHA:         commit.ID.String(),
			IsForkPullRequest: pr != nil && pr.IsFromFork(),
			Event:             evt,
			EventPayload:      string(p),
			Status:            bots_model.StatusWaiting,
		}
		if len(run.Title) > 255 {
			run.Title = run.Title[:255] // FIXME: we should use a better method to cut title
		}
		jobs, err := jobparser.Parse(content)
		if err != nil {
			log.Error("jobparser.Parse: %v", err)
			continue
		}
		if err := bots_model.InsertRun(&run, jobs); err != nil {
			log.Error("InsertRun: %v", err)
		}
	}
	return nil
}

// NotifyNewIssue notifies issue created event
func (a *botsNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	if err := notify(issue.Repo, issue.Poster, issue.Repo.DefaultBranch,
		webhook.HookEventIssues, &api.IssuePayload{
			Action:     api.HookIssueOpened,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(issue.Poster, nil),
		}); err != nil {
		log.Error("notify: %v", err)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (a *botsNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	mode, _ := access_model.AccessLevel(ctx, issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(ctx); err != nil {
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
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventPullRequest, apiPullRequest)
	} else {
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
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	}
}

func (a *botsNotifier) NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	_, _ []*issues_model.Label,
) {
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
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(ctx, issue.PullRequest, nil),
			Repository:  convert.ToRepo(issue.Repo, perm.AccessModeNone),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("bostNotifier [is_pull: %v]: %v", issue.IsPull, err)
	}
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (a *botsNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	mode, _ := access_model.AccessLevel(ctx, doer, repo)

	var err error
	if issue.IsPull {
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = notify(issue.Repo, doer, issue.Repo.DefaultBranch, webhook.HookEventIssueComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(ctx, issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (a *botsNotifier) NotifyNewPullRequest(ctx context.Context, pull *issues_model.PullRequest, mentions []*user_model.User) {
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
	if err := notifyWithPR(pull.Issue.Repo, pull.Issue.Poster, pull.Issue.Repo.DefaultBranch, webhook.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(ctx, pull, nil),
		Repository:  convert.ToRepo(pull.Issue.Repo, mode),
		Sender:      convert.ToUser(pull.Issue.Poster, nil),
	}, pull); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (a *botsNotifier) NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

func (a *botsNotifier) NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

func (a *botsNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := notify(repo, doer, repo.DefaultBranch,
		webhook.HookEventRepository,
		&api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
			Organization: convert.ToUser(u, nil),
			Sender:       convert.ToUser(doer, nil),
		}); err != nil {
		log.Error("Bots Notifier [repo_id: %d]: %v", repo.ID, err)
	}
}

func (a *botsNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	oldMode, _ := access_model.AccessLevel(ctx, doer, oldRepo)
	mode, _ := access_model.AccessLevel(ctx, doer, repo)

	// forked webhook
	if err := notify(oldRepo, doer, oldRepo.DefaultBranch, webhook.HookEventFork, &api.ForkPayload{
		Forkee: convert.ToRepo(oldRepo, oldMode),
		Repo:   convert.ToRepo(repo, mode),
		Sender: convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	}

	u := repo.MustOwner(ctx)

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := notify(repo, doer, oldRepo.DefaultBranch, webhook.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(repo, perm.AccessModeOwner),
			Organization: convert.ToUser(u, nil),
			Sender:       convert.ToUser(doer, nil),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
	}
}

func (a *botsNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
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
	if err := notifyWithPR(review.Issue.Repo, review.Reviewer, review.CommitID, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueReviewed,
		Index:       review.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(db.DefaultContext, pr, nil),
		Repository:  convert.ToRepo(review.Issue.Repo, mode),
		Sender:      convert.ToUser(review.Reviewer, nil),
		Review: &api.ReviewPayload{
			Type:    string(reviewHookType),
			Content: review.Content,
		},
	}, pr); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (*botsNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
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

	err = notifyWithPR(pr.Issue.Repo, doer, pr.MergedCommitID, webhook.HookEventPullRequest, apiPullRequest, pr)
	if err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (a *botsNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := notify(repo, pusher, opts.RefFullName, webhook.HookEventPush, &api.PushPayload{
		Ref:        opts.RefFullName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + commits.CompareURL,
		Commits:    apiCommits,
		HeadCommit: apiHeadCommit,
		Repo:       convert.ToRepo(repo, perm.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (a *botsNotifier) NotifyCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := notify(repo, pusher, refName, webhook.HookEventCreate, &api.CreatePayload{
		Ref:     refName,
		Sha:     refID,
		RefType: refType,
		Repo:    apiRepo,
		Sender:  apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (a *botsNotifier) NotifyDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, perm.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := notify(repo, pusher, refName, webhook.HookEventDelete, &api.DeletePayload{
		Ref:        refName,
		RefType:    refType,
		PusherType: api.PusherTypeUser,
		Repo:       apiRepo,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks.(delete %s): %v", refType, err)
	}
}

func (a *botsNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(db.DefaultContext, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := notify(repo, pusher, apiHeadCommit.ID, webhook.HookEventPush, &api.PushPayload{
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
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (a *botsNotifier) NotifySyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	a.NotifyCreateRef(ctx, pusher, repo, refType, refFullName, refID)
}

func (a *botsNotifier) NotifySyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	a.NotifyDeleteRef(ctx, pusher, repo, refType, refFullName)
}

func sendReleaseNofiter(ctx context.Context, doer *user_model.User, rel *repo_model.Release, ref string, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := access_model.AccessLevel(ctx, doer, rel.Repo)
	if err := notify(rel.Repo, doer, ref, webhook.HookEventRelease, &api.ReleasePayload{
		Action:     action,
		Release:    convert.ToRelease(rel),
		Repository: convert.ToRepo(rel.Repo, mode),
		Sender:     convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("notify: %v", err)
	}
}

func (a *botsNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	sendReleaseNofiter(ctx, rel.Publisher, rel, rel.Sha1, api.HookReleasePublished)
}

func (a *botsNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseNofiter(ctx, doer, rel, rel.Sha1, api.HookReleaseUpdated)
}

func (a *botsNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	sendReleaseNofiter(ctx, doer, rel, rel.Sha1, api.HookReleaseDeleted)
}

func (a *botsNotifier) NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(doer, pd, api.HookPackageCreated)
}

func (a *botsNotifier) NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	notifyPackage(doer, pd, api.HookPackageDeleted)
}

func notifyPackage(sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	if pd.Repository == nil {
		// TODO https://github.com/go-gitea/gitea/pull/17940
		return
	}

	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.notifyPackage Package: %s[%d]", pd.Package.Name, pd.Package.ID))
	defer finished()

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	if err := notify(pd.Repository, sender, "", webhook.HookEventPackage, &api.PackagePayload{
		Action:  action,
		Package: apiPackage,
		Sender:  convert.ToUser(sender, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}
