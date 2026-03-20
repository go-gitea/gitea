// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package eventsourcenotifier broadcasts repo activity events to connected
// users via the existing Server-Sent Events infrastructure so that issue and
// pull-request pages can show a non-intrusive "New activity" banner without
// requiring a manual page refresh.
package eventsourcenotifier

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
)

type eventsourceNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &eventsourceNotifier{}

// Init registers the eventsource notifier with the notify system.
func Init() error {
	notify_service.RegisterNotifier(&eventsourceNotifier{})
	return nil
}

// repoActivityPayload is the JSON payload sent to the browser.
type repoActivityPayload struct {
	RepoID     int64 `json:"repoID"`
	IssueIndex int64 `json:"issueIndex,omitempty"`
}

// sendRepoActivity pushes a repo-activity SSE event to every watcher of the
// repository that currently has an open /user/events connection.
func sendRepoActivity(ctx context.Context, repoID, issueIndex int64) {
	watcherIDs, err := repo_model.GetRepoWatchersIDs(ctx, repoID)
	if err != nil {
		log.Error("eventsourceNotifier: GetRepoWatchersIDs(%d): %v", repoID, err)
		return
	}

	payload := repoActivityPayload{RepoID: repoID, IssueIndex: issueIndex}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error("eventsourceNotifier: json.Marshal: %v", err)
		return
	}

	mgr := eventsource.GetManager()
	event := &eventsource.Event{
		Name: "repo-activity",
		Data: string(data),
	}
	for _, uid := range watcherIDs {
		mgr.SendMessage(uid, event)
	}
}

// CreateIssueComment fires when a comment is posted on an issue or PR.
func (n *eventsourceNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User,
	repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment,
	mentions []*user_model.User,
) {
	sendRepoActivity(ctx, repo.ID, issue.Index)
}

// NewIssue fires when a new issue is opened.
func (n *eventsourceNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("eventsourceNotifier: NewIssue LoadRepo: %v", err)
		return
	}
	sendRepoActivity(ctx, issue.RepoID, 0)
}

// IssueChangeStatus fires when an issue is closed or reopened.
func (n *eventsourceNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User,
	commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool,
) {
	sendRepoActivity(ctx, issue.RepoID, issue.Index)
}

// PushCommits fires when commits are pushed to a repository.
func (n *eventsourceNotifier) PushCommits(ctx context.Context, pusher *user_model.User,
	repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits,
) {
	sendRepoActivity(ctx, repo.ID, 0)
}
