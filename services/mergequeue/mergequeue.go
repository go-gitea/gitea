// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

var branchQueue atomic.Pointer[queue.WorkerPoolQueue[string]]

// Init leaves existing queues to the cron task, the server can't answer the git hooks of a merge yet
func Init() error {
	notify_service.RegisterNotifier(&notifier{})
	worker := queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "merge_queue", handleBranches)
	if worker == nil {
		return errors.New("unable to create merge_queue queue")
	}
	branchQueue.Store(worker)
	go graceful.GetManager().RunWithCancel(worker)
	return nil
}

func CheckAllQueues(ctx context.Context) error {
	branches, err := pull_model.GetMergeQueueBranches(ctx)
	if err != nil {
		return err
	}
	for _, branch := range branches {
		addToQueue(branch.RepoID, branch.BaseBranch)
	}
	return nil
}

func addToQueue(repoID int64, branch string) {
	worker := branchQueue.Load()
	if worker == nil {
		return
	}
	if err := worker.Push(fmt.Sprintf("%d:%s", repoID, branch)); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Unable to push merge queue of repo %d branch %s: %v", repoID, branch, err)
	}
}

func handleBranches(items ...string) []string {
	for _, item := range items {
		idStr, branch, _ := strings.Cut(item, ":")
		repoID, _ := strconv.ParseInt(idStr, 10, 64)
		ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), "merge queue "+item)
		err := globallock.LockAndDo(ctx, lockKey(repoID, branch), func(ctx context.Context) error {
			return processQueue(ctx, repoID, branch)
		})
		if err != nil {
			log.Error("Merge queue of repo %d branch %s: %v", repoID, branch, err)
		}
		finished()
	}
	return nil
}

func lockKey(repoID int64, branch string) string {
	return fmt.Sprintf("merge_queue_%d_%s", repoID, branch)
}

type queueState struct {
	repo    *repo_model.Repository
	gitRepo *git.Repository
	rule    *git_model.ProtectedBranch
	branch  string
	entries []*QueueEntry
}

func processQueue(ctx context.Context, repoID int64, branch string) error {
	rows, err := pull_model.GetMergeQueueEntries(ctx, repoID, branch)
	if err != nil || len(rows) == 0 {
		return err
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	rule, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repoID, branch)
	if err != nil {
		return err
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return err
	}
	defer closer.Close()
	entries, err := loadEntries(ctx, rows, repo)
	if err != nil {
		return err
	}
	if err := repo.LoadUnits(ctx); err != nil {
		return err
	}
	s := &queueState{repo: repo, gitRepo: gitRepo, rule: rule, branch: branch, entries: entries}
	if rule == nil || !rule.RequireMergeQueue || !repo.AllowsPulls(ctx) {
		return s.removeEntries(ctx, issues_model.MergeQueueRemovedNotMergeable, entries...)
	}
	if err := s.dropStaleEntries(ctx); err != nil || len(s.entries) == 0 {
		return err
	}
	base, err := gitRepo.GetBranchCommitID(ctx, branch)
	if git.IsErrNotExist(err) {
		return s.removeEntries(ctx, issues_model.MergeQueueRemovedBaseDeleted, s.entries...)
	} else if err != nil {
		return err
	}
	if err := s.evaluateChecks(ctx, base); err != nil {
		return err
	}
	if merged, err := s.mergeReadyEntries(ctx); err != nil || merged {
		return err // the push to the base branch schedules the next step
	}
	return s.buildEntries(ctx, base)
}

func (s *queueState) dropStaleEntries(ctx context.Context) error {
	live := s.entries[:0]
	for _, entry := range s.entries {
		var reason issues_model.MergeQueueRemovalReason
		switch pr := entry.PullRequest; {
		case pr == nil:
			reason = issues_model.MergeQueueRemovedClosed
		case pr.HasMerged:
			reason = issues_model.MergeQueueRemovedAlreadyMerged
		case pr.BaseBranch != s.branch:
			reason = issues_model.MergeQueueRemovedBaseChanged
		case pr.Issue.IsClosed:
			reason = issues_model.MergeQueueRemovedClosed
		default:
			head, err := s.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
			if err != nil {
				return err
			} else if head == entry.HeadSHA {
				live = append(live, entry)
				continue
			}
			reason = issues_model.MergeQueueRemovedHeadChanged
		}
		if err := s.removeEntries(ctx, reason, entry); err != nil {
			return err
		}
	}
	s.entries = live
	return nil
}

func (s *queueState) isBlocked(ctx context.Context, entry *QueueEntry) (bool, error) {
	if entry.Enqueuer.IsGhost() {
		return true, nil
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, s.repo, entry.Enqueuer)
	if err != nil {
		return false, err
	}
	if !git_model.IsUserMergeWhitelisted(ctx, s.rule, entry.Enqueuer.ID, perm) {
		return true, nil
	}
	pr := entry.PullRequest
	if noDeps, err := issues_model.IssueNoDependenciesLeft(ctx, pr.Issue); err != nil || !noDeps {
		return !noDeps, err
	}
	return pr.IsWorkInProgress(ctx) || pull_service.CheckPullReviews(ctx, s.rule, pr) != nil, nil
}

func (s *queueState) validateChain(ctx context.Context, base string) ([]*QueueEntry, error) {
	for i, entry := range s.entries {
		if !entry.IsBuilt() || entry.BaseSHA != base {
			return s.entries[:i], s.invalidateFrom(ctx, i)
		}
		base = entry.GroupHeadSHA
	}
	return s.entries, nil
}

func (s *queueState) evaluateChecks(ctx context.Context, base string) error {
	built, err := s.validateChain(ctx, base)
	if err != nil || len(built) == 0 {
		return err
	}
	required, err := pull_service.EffectiveRequiredContexts(ctx, s.repo, s.rule)
	if err != nil {
		return err
	}
	required = pull_service.MergeQueueRequiredContexts(required)
	cfg := s.rule.MergeQueue
	timeout := time.Duration(cfg.CheckResponseTimeoutMinutes) * time.Minute
	firstFailed := -1
	var failureReason issues_model.MergeQueueRemovalReason
	for i, entry := range built {
		statuses, err := pull_service.GetLatestPullCommitStatuses(ctx, s.repo.ID, entry.GroupHeadSHA)
		if err != nil {
			return err
		}
		nextState := pull_model.MergeQueueStateAwaitingChecks
		var reason issues_model.MergeQueueRemovalReason
		switch state := pull_service.MergeRequiredContextsCommitStatus(statuses, required); {
		case state.IsSuccess():
			nextState = pull_model.MergeQueueStateMergeable
		case state.IsFailure() || state.IsError():
			reason = issues_model.MergeQueueRemovedChecksFailed
		case entry.ChecksStartedUnix.AddDuration(timeout) <= timeutil.TimeStampNow():
			reason = issues_model.MergeQueueRemovedChecksTimeout
		}
		if reason != "" {
			nextState = pull_model.MergeQueueStateUnmergeable
			if firstFailed < 0 {
				firstFailed, failureReason = i, reason
			}
		} else if cfg.GroupingStrategy == git_model.MergeQueueGroupingHeadGreen && i < cfg.MaxEntriesToMerge {
			firstFailed = -1 // This group can still merge the failed entries before it.
		}
		if entry.State != nextState {
			entry.State = nextState
			if err := pull_model.UpdateMergeQueueEntry(ctx, entry.MergeQueueEntry); err != nil {
				return err
			}
		}
	}
	if firstFailed >= 0 {
		return s.dropEntry(ctx, firstFailed, failureReason)
	}
	return nil
}

func (s *queueState) invalidateFrom(ctx context.Context, index int) error {
	for _, entry := range s.entries[index:] {
		if !entry.IsBuilt() {
			continue
		}
		destroyGroup(ctx, s.repo, entry.Enqueuer, entry.MergeQueueEntry, "invalidated")
		entry.State = pull_model.MergeQueueStateQueued
		entry.BaseSHA, entry.GroupHeadSHA, entry.GroupRef, entry.ChecksStartedUnix = "", "", "", 0
		if err := pull_model.UpdateMergeQueueEntry(ctx, entry.MergeQueueEntry); err != nil {
			return err
		}
	}
	return nil
}

func (s *queueState) dropEntry(ctx context.Context, index int, reason issues_model.MergeQueueRemovalReason) error {
	if err := s.removeEntries(ctx, reason, s.entries[index]); err != nil {
		return err
	}
	s.entries = slices.Delete(s.entries, index, index+1)
	if err := s.invalidateFrom(ctx, index); err != nil {
		return err
	}
	addToQueue(s.repo.ID, s.branch)
	return nil
}

func (s *queueState) mergeReadyEntries(ctx context.Context) (bool, error) {
	cfg := s.rule.MergeQueue
	var group []*QueueEntry
	for i, entry := range s.entries {
		if i >= cfg.MaxEntriesToMerge || !entry.IsBuilt() {
			break
		}
		if entry.State == pull_model.MergeQueueStateMergeable {
			group = s.entries[:i+1]
		} else if cfg.GroupingStrategy != git_model.MergeQueueGroupingHeadGreen {
			break
		}
	}
	wait := time.Duration(cfg.MinEntriesToMergeWaitMinutes) * time.Minute
	if len(group) == 0 || (len(group) < cfg.MinEntriesToMerge && group[0].CreatedUnix.AddDuration(wait) > timeutil.TimeStampNow()) {
		return false, nil
	}
	items := make([]pull_service.MergeQueueItem, len(group))
	for i, entry := range group {
		if blocked, err := s.isBlocked(ctx, entry); err != nil {
			return false, err
		} else if blocked {
			return false, s.dropEntry(ctx, i, issues_model.MergeQueueRemovedNotMergeable)
		}
		items[i] = pull_service.MergeQueueItem{PR: entry.PullRequest, Doer: entry.Enqueuer, HeadSHA: entry.HeadSHA, CommitID: entry.GroupHeadSHA}
	}
	if err := pull_service.MergeQueuePullRequests(ctx, s.repo, s.branch, items); err != nil {
		if _, ok := errors.AsType[*git.ErrPushRejected](err); ok { // unlike an out-of-date push, a rejection repeats on every retry
			last := len(group) - 1
			log.Warn("Merge queue push of %-v rejected: %v", group[last].PullRequest, err)
			return false, s.dropEntry(ctx, last, issues_model.MergeQueueRemovedNotMergeable)
		}
		return false, err
	}
	for _, entry := range group {
		pr, doer := entry.PullRequest, entry.Enqueuer
		if deleted, err := pull_model.DeleteMergeQueueEntry(ctx, entry.PullID); err != nil {
			log.Error("DeleteMergeQueueEntry: %v", err)
		} else if deleted && pr.HasMerged {
			notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, issues_model.MergeQueueRemovedMerged)
		}
		destroyGroup(ctx, s.repo, doer, entry.MergeQueueEntry, "merged")
		if shouldDelete, err := pull_service.ShouldDeleteBranchAfterMerge(ctx, nil, s.repo, pr); err != nil {
			log.Error("ShouldDeleteBranchAfterMerge: %v", err)
		} else if shouldDelete {
			if err := repo_service.DeleteBranchAfterMerge(ctx, doer, pr.ID, nil); err != nil {
				log.Error("DeleteBranchAfterMerge: %v", err)
			}
		}
	}
	return true, nil
}

func (s *queueState) buildEntries(ctx context.Context, base string) error {
	building, scheduled := 0, false
	for _, entry := range s.entries {
		if !entry.IsBuilt() {
			if building >= s.rule.MergeQueue.MaxEntriesToBuild {
				break
			}
			reason, err := s.buildEntry(ctx, entry, base)
			if err != nil {
				return err
			} else if reason != "" {
				if err := s.removeEntries(ctx, reason, entry); err != nil {
					return err
				}
				addToQueue(s.repo.ID, s.branch)
				continue
			}
			scheduled = true
		}
		if entry.State == pull_model.MergeQueueStateAwaitingChecks {
			building++
		}
		base = entry.GroupHeadSHA
	}
	if scheduled {
		addToQueue(s.repo.ID, s.branch)
	}
	return nil
}

func (s *queueState) buildEntry(ctx context.Context, entry *QueueEntry, base string) (issues_model.MergeQueueRemovalReason, error) {
	if blocked, err := s.isBlocked(ctx, entry); err != nil {
		return "", err
	} else if blocked {
		return issues_model.MergeQueueRemovedNotMergeable, nil
	}
	pr := entry.PullRequest
	ref := pull_model.MergeQueueGroupRef(s.branch, pr.Index, base)
	sha, err := pull_service.CreateMergeQueueCommit(ctx, pr, entry.Enqueuer, s.gitRepo, s.rule.MergeQueue.MergeMethod, base, entry.HeadSHA, ref)
	if err != nil {
		switch mergeErr := err.(type) {
		case pull_service.ErrSHADoesNotMatch:
			return issues_model.MergeQueueRemovedHeadChanged, nil
		case pull_service.ErrMergeQueueMerge:
			cause := mergeErr.Unwrap()
			switch {
			case pull_service.IsErrMergeConflicts(cause) || pull_service.IsErrRebaseConflicts(cause) || pull_service.IsErrMergeUnrelatedHistories(cause):
				return issues_model.MergeQueueRemovedConflict, nil
			case gitcmd.IsErrorExitCode(cause, 1) || errors.Is(cause, util.ErrInvalidArgument):
				log.Error("CreateMergeQueueCommit %-v: %v", pr, cause)
				return issues_model.MergeQueueRemovedNotMergeable, nil
			}
		}
		return "", err
	}
	entry.State = pull_model.MergeQueueStateAwaitingChecks
	entry.BaseSHA, entry.GroupHeadSHA, entry.GroupRef = base, sha, ref
	entry.ChecksStartedUnix = timeutil.TimeStampNow()
	if err := pull_model.UpdateMergeQueueEntry(ctx, entry.MergeQueueEntry); err != nil {
		return "", err
	}
	notify_service.MergeGroupChanged(ctx, entry.Enqueuer, s.repo, entry.MergeQueueEntry, api.HookMergeGroupChecksRequested, "")
	return "", nil
}

func destroyGroup(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, entry *pull_model.MergeQueueEntry, reason string) {
	if !entry.IsBuilt() {
		return
	}
	if err := git.RemoveRef(ctx, repo, entry.GroupRef); err != nil {
		log.Error("RemoveRef %s: %v", entry.GroupRef, err)
	}
	notify_service.MergeGroupChanged(ctx, doer, repo, entry, api.HookMergeGroupDestroyed, reason)
}

func (s *queueState) removeEntries(ctx context.Context, reason issues_model.MergeQueueRemovalReason, entries ...*QueueEntry) error {
	for _, entry := range entries {
		if err := removeEntry(ctx, s.repo, entry.MergeQueueEntry, entry.PullRequest, entry.Enqueuer, reason); err != nil {
			return err
		}
	}
	return nil
}

func removeEntry(ctx context.Context, repo *repo_model.Repository, entry *pull_model.MergeQueueEntry, pr *issues_model.PullRequest, doer *user_model.User, reason issues_model.MergeQueueRemovalReason) error {
	deleted, err := db.WithTx2(ctx, func(ctx context.Context) (bool, error) {
		deleted, err := pull_model.DeleteMergeQueueEntry(ctx, entry.PullID)
		if err != nil || !deleted || pr == nil {
			return deleted, err
		}
		return true, issues_model.CreateMergeQueueComment(ctx, pr, doer, reason)
	})
	if err != nil || !deleted {
		return err
	}
	destroyGroup(ctx, repo, doer, entry, "dequeued")
	if pr != nil {
		notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, reason)
	}
	return nil
}
