// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/models"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	bots_service "code.gitea.io/gitea/services/bots"
)

type botsNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &botsNotifier{}

// NewNotifier create a new botsNotifier notifier
func NewNotifier() base.Notifier {
	return &botsNotifier{}
}

func detectWorkflows(commit *git.Commit, event webhook.HookEventType, ref string) (bool, error) {
	tree, err := commit.SubTree(".github/workflows")
	if _, ok := err.(git.ErrNotExist); ok {
		tree, err = commit.SubTree(".gitea/workflows")
	}
	if _, ok := err.(git.ErrNotExist); ok {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return false, err
	}

	log.Trace("detected %s has %d entries", commit.ID, len(entries))

	return len(entries) > 0, nil
}

func notifyIssue(issue *models.Issue, doer *user_model.User, evt webhook.HookEventType, payload string) {
	err := issue.LoadRepo(db.DefaultContext)
	if err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if issue.Repo.IsEmpty || issue.Repo.IsArchived {
		return
	}

	ref := issue.Ref
	if ref == "" {
		ref = issue.Repo.DefaultBranch
	}

	gitRepo, err := git.OpenRepository(context.Background(), issue.Repo.RepoPath())
	if err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		log.Error("gitRepo.GetCommit: %v", err)
		return
	}

	hasWorkflows, err := detectWorkflows(commit, evt, ref)
	if err != nil {
		log.Error("detectWorkflows: %v", err)
		return
	}
	if !hasWorkflows {
		log.Trace("repo %s with commit %s couldn't find workflows", issue.Repo.RepoPath(), commit.ID)
		return
	}

	task := bots_model.Task{
		Title:         commit.CommitMessage,
		RepoID:        issue.RepoID,
		TriggerUserID: doer.ID,
		Event:         evt,
		EventPayload:  payload,
		Status:        bots_model.TaskPending,
		Ref:           ref,
		CommitSHA:     commit.ID.String(),
	}

	if err := bots_model.InsertTask(&task); err != nil {
		log.Error("InsertBotTask: %v", err)
	} else {
		bots_service.PushToQueue(&task)
	}
}

// TODO: implement all events
func (a *botsNotifier) NotifyNewIssue(issue *models.Issue, mentions []*user_model.User) {
	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"number": issue.Index,
		},
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("NotifyNewIssue: %v", err)
		return
	}
	notifyIssue(issue, issue.Poster, webhook.HookEventIssues, string(bs))
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (a *botsNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *models.Issue, actionComment *models.Comment, closeOrReopen bool) {
}

func (a *botsNotifier) NotifyIssueChangeLabels(doer *user_model.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label,
) {
	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"number": issue.Index,
		},
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("NotifyNewIssue: %v", err)
		return
	}
	notifyIssue(issue, doer, webhook.HookEventIssueLabel, string(bs))
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (a *botsNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*user_model.User) {
}

func (a *botsNotifier) NotifyNewPullRequest(pull *models.PullRequest, mentions []*user_model.User) {
}

func (a *botsNotifier) NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

func (a *botsNotifier) NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

func (a *botsNotifier) NotifyCreateRepository(doer *user_model.User, u *user_model.User, repo *repo_model.Repository) {
}

func (a *botsNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
}

func (a *botsNotifier) NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*user_model.User) {
}

func (*botsNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *user_model.User) {
}

func (a *botsNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(commits.HeadCommit.Sha1)
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	hasWorkflows, err := detectWorkflows(commit, webhook.HookEventPush, opts.RefFullName)
	if err != nil {
		log.Error("detectWorkflows: %v", err)
		return
	}
	if !hasWorkflows {
		log.Trace("repo %s with commit %s couldn't find workflows", repo.RepoPath(), commit.ID)
		return
	}

	payload := &api.PushPayload{
		Ref:        opts.RefFullName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + commits.CompareURL,
		Commits:    apiCommits,
		HeadCommit: apiHeadCommit,
		Repo:       convert.ToRepo(repo, perm.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}

	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("json.Marshal(payload) failed: %v", err)
		return
	}

	task := bots_model.Task{
		Title:         commit.Message(),
		RepoID:        repo.ID,
		TriggerUserID: pusher.ID,
		Event:         webhook.HookEventPush,
		EventPayload:  string(bs),
		Status:        bots_model.TaskPending,
	}

	if err := bots_model.InsertTask(&task); err != nil {
		log.Error("InsertBotTask: %v", err)
	} else {
		bots_service.PushToQueue(&task)
	}
}

func (a *botsNotifier) NotifyCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

func (a *botsNotifier) NotifyDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

func (a *botsNotifier) NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

func (a *botsNotifier) NotifySyncCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

func (a *botsNotifier) NotifySyncDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

func (a *botsNotifier) NotifyNewRelease(rel *models.Release) {
}
