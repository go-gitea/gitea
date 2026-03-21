// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package eventsourcenotifier broadcasts repo activity events to connected
// users via the existing Server-Sent Events infrastructure so that issue and
// pull-request pages can show a non-intrusive "New activity" banner without
// requiring a manual page refresh.
package eventsourcenotifier

import (
	"context"
	"sync"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
)

// debounceDelay collapses rapid consecutive events for the same repository
// into a single SSE signal to avoid flooding watchers during busy periods.
const debounceDelay = 500 * time.Millisecond

// repoActivityPayload is the JSON payload sent to the browser.
type repoActivityPayload struct {
	RepoID     int64  `json:"repoID"`
	IssueIndex int64  `json:"issueIndex,omitempty"`
	EventType  string `json:"eventType"`
}

// debounceEntry holds the pending broadcast state for a single repository.
type debounceEntry struct {
	timer      *time.Timer
	payload    repoActivityPayload
	watcherIDs []int64
	generation uint64
}

var (
	debounceMu sync.Mutex
	debounced  = make(map[int64]*debounceEntry)
)

type eventsourceNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &eventsourceNotifier{}

// Init registers the eventsource notifier with the notify system.
// It is a no-op when [ui.notification] ENABLE_REPO_ACTIVITY_EVENTS = false.
func Init() error {
	if !setting.UI.Notification.RepoActivityEvents {
		return nil
	}
	notify_service.RegisterNotifier(&eventsourceNotifier{})
	return nil
}

// scheduleRepoActivity debounces and then broadcasts a repo-activity SSE event
// to all watchers of the repository. Rapid consecutive calls for the same repo
// within debounceDelay are collapsed into a single broadcast.
func scheduleRepoActivity(ctx context.Context, repoID, issueIndex int64, eventType string) {
	watcherIDs, err := repo_model.GetRepoWatchersIDs(ctx, repoID)
	if err != nil {
		log.Error("eventsourceNotifier: GetRepoWatchersIDs(%d): %v", repoID, err)
		return
	}
	if len(watcherIDs) == 0 {
		return
	}

	payload := repoActivityPayload{RepoID: repoID, IssueIndex: issueIndex, EventType: eventType}

	debounceMu.Lock()
	entry, exists := debounced[repoID]
	if !exists {
		entry = &debounceEntry{}
		debounced[repoID] = entry
	} else {
		entry.timer.Stop()
		// Prefer the more specific payload (issue-scoped over repo-wide).
		if payload.IssueIndex == 0 && entry.payload.IssueIndex != 0 {
			payload.IssueIndex = entry.payload.IssueIndex
		}
	}
	entry.payload = payload
	entry.watcherIDs = watcherIDs
	entry.generation++
	gen := entry.generation
	entry.timer = time.AfterFunc(debounceDelay, func() {
		broadcastDebounced(repoID, gen)
	})
	debounceMu.Unlock()
}

// broadcastDebounced sends the SSE event if this generation is still current.
func broadcastDebounced(repoID int64, gen uint64) {
	debounceMu.Lock()
	entry, ok := debounced[repoID]
	if !ok || entry.generation != gen {
		debounceMu.Unlock()
		return // superseded by a newer event
	}
	payload := entry.payload
	watcherIDs := entry.watcherIDs
	delete(debounced, repoID)
	debounceMu.Unlock()

	data, err := json.Marshal(payload)
	if err != nil {
		log.Error("eventsourceNotifier: json.Marshal: %v", err)
		return
	}

	mgr := eventsource.GetManager()
	event := &eventsource.Event{Name: "repo-activity", Data: string(data)}
	for _, uid := range watcherIDs {
		mgr.SendMessage(uid, event)
	}
}

// CreateIssueComment fires when a comment is posted on an issue or PR.
func (n *eventsourceNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User,
	repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment,
	mentions []*user_model.User,
) {
	scheduleRepoActivity(ctx, repo.ID, issue.Index, "comment")
}

// NewIssue fires when a new issue is opened.
func (n *eventsourceNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("eventsourceNotifier: NewIssue LoadRepo: %v", err)
		return
	}
	scheduleRepoActivity(ctx, issue.RepoID, issue.Index, "issue-opened")
}

// IssueChangeStatus fires when an issue or PR is closed or reopened.
func (n *eventsourceNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User,
	commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool,
) {
	eventType := "issue-reopened"
	if closeOrReopen {
		eventType = "issue-closed"
	}
	scheduleRepoActivity(ctx, issue.RepoID, issue.Index, eventType)
}

// MergePullRequest fires when a pull request is merged.
func (n *eventsourceNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("eventsourceNotifier: MergePullRequest LoadIssue: %v", err)
		return
	}
	scheduleRepoActivity(ctx, pr.BaseRepoID, pr.Issue.Index, "merged")
}

// PullRequestReview fires when a review (approve / request-changes / comment) is submitted.
func (n *eventsourceNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest,
	review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User,
) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("eventsourceNotifier: PullRequestReview LoadIssue: %v", err)
		return
	}
	scheduleRepoActivity(ctx, pr.BaseRepoID, pr.Issue.Index, "review")
}

// PullRequestCodeComment fires when an inline code comment is posted on a PR.
func (n *eventsourceNotifier) PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest,
	comment *issues_model.Comment, mentions []*user_model.User,
) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("eventsourceNotifier: PullRequestCodeComment LoadIssue: %v", err)
		return
	}
	scheduleRepoActivity(ctx, pr.BaseRepoID, pr.Issue.Index, "review-comment")
}

// NewRelease fires when a release is published.
func (n *eventsourceNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	scheduleRepoActivity(ctx, rel.RepoID, 0, "release")
}

// PushCommits fires when commits are pushed to a repository.
func (n *eventsourceNotifier) PushCommits(ctx context.Context, pusher *user_model.User,
	repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits,
) {
	scheduleRepoActivity(ctx, repo.ID, 0, "push")
}
