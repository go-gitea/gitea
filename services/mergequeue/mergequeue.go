// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"

	"xorm.io/builder"
)

var branchQueue atomic.Pointer[queue.WorkerPoolQueue[pull_model.MergeQueueBranch]]

func Init() error {
	if ref := pull_model.MergeQueueRefPrefix() + "main"; !git.IsValidRefPattern(ref) {
		return fmt.Errorf("invalid [repository.pull-request] MERGE_QUEUE_BRANCH_PREFIX, %q is not a valid ref", ref)
	}
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
	return scheduleQueues(ctx, builder.NewCond())
}

func scheduleQueues(ctx context.Context, cond builder.Cond) error {
	branches, err := pull_model.GetMergeQueueBranches(ctx, cond)
	if err != nil {
		return err
	}
	for _, branch := range branches {
		addToQueue(branch.RepoID, branch.Branch)
	}
	return nil
}

func addToQueue(repoID int64, branch string) {
	worker := branchQueue.Load()
	if worker == nil {
		return
	}
	if err := worker.Push(pull_model.MergeQueueBranch{RepoID: repoID, Branch: branch}); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		log.Error("Unable to push merge queue of repo %d branch %s: %v", repoID, branch, err)
	}
}

func handleBranches(items ...pull_model.MergeQueueBranch) []pull_model.MergeQueueBranch {
	for _, item := range items {
		handleBranch(item)
	}
	return nil
}

func handleBranch(item pull_model.MergeQueueBranch) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("merge queue of repo %d branch %s", item.RepoID, item.Branch))
	defer finished()
	err := globallock.LockAndDo(ctx, pull_model.MergeQueueLockKey(item.RepoID, item.Branch), func(ctx context.Context) error {
		return processQueue(ctx, item.RepoID, item.Branch)
	})
	if err != nil {
		log.Error("Merge queue of repo %d branch %s: %v", item.RepoID, item.Branch, err)
	}
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
	state := &queueState{repo: repo, gitRepo: gitRepo, rule: rule, branch: branch, entries: entries}
	if err := state.dropStaleEntries(ctx); err != nil || len(state.entries) == 0 {
		return err
	}
	base, err := gitRepo.GetBranchCommitID(ctx, branch)
	if git.IsErrNotExist(err) {
		return state.removeEntries(ctx, issues_model.MergeQueueRemovedBaseDeleted, state.entries...)
	} else if err != nil {
		return err
	}
	if merged, err := state.mergeReadyEntries(ctx, base); err != nil || merged {
		return err // the base push schedules the next queue pass
	}
	return state.buildEntries(ctx, base)
}

func (s *queueState) dropStaleEntries(ctx context.Context) error {
	disabled := s.rule == nil || !s.rule.RequireMergeQueue || !s.repo.AllowsPulls(ctx)
	live := s.entries[:0]
	for _, entry := range s.entries {
		if err := entry.RestoreInterruptedMerge(ctx); err != nil {
			return err
		}
		pr := entry.PullRequest
		var reason issues_model.MergeQueueRemovalReason
		switch {
		case pr == nil:
			reason = issues_model.MergeQueueRemovedClosed
		case pr.HasMerged:
			reason = util.Iif(pr.MergedCommitID == entry.GroupHeadSHA, issues_model.MergeQueueRemovedMerged, issues_model.MergeQueueRemovedAlreadyMerged)
		case disabled:
			reason = issues_model.MergeQueueRemovedNotMergeable
		case pr.BaseBranch != s.branch:
			reason = issues_model.MergeQueueRemovedBaseChanged
		case pr.Issue.IsClosed:
			reason = issues_model.MergeQueueRemovedClosed
		default:
			if err := pr.LoadHeadRepo(ctx); err != nil {
				return err
			}
			reason = issues_model.MergeQueueRemovedHeadChanged
			if pr.HeadRepo == nil {
				break
			}
			head, err := s.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
			if err != nil && !git.IsErrNotExist(err) {
				return err
			} else if err == nil && head == entry.HeadSHA {
				live = append(live, entry)
				continue
			}
		}
		if err := s.removeEntries(ctx, reason, entry); err != nil {
			return err
		}
	}
	s.entries = live
	return nil
}

func (s *queueState) validateChain(ctx context.Context, base string) ([]*QueueEntry, error) {
	for i, entry := range s.entries {
		if entry.BaseSHA != base || entry.GroupRef != pull_model.MergeQueueGroupRef(s.branch, entry.PullRequest.Index, base) {
			return s.entries[:i], s.invalidateFrom(ctx, i)
		}
		base = entry.GroupHeadSHA
	}
	return s.entries, nil
}

func (s *queueState) readyGroup(ctx context.Context, base string) ([]*QueueEntry, error) {
	built, err := s.validateChain(ctx, base)
	if err != nil || len(built) == 0 {
		return nil, err
	}
	required, err := pull_service.EffectiveRequiredContexts(ctx, s.repo, s.rule)
	if err != nil {
		return nil, err
	}
	required = pull_service.MergeQueueRequiredContexts(required)
	cfg := s.rule.MergeQueue
	timeout := time.Duration(cfg.CheckResponseTimeoutMinutes) * time.Minute
	headGreen := cfg.GroupingStrategy == git_model.MergeQueueGroupingHeadGreen
	var ready, failedAt int
	var failureReason issues_model.MergeQueueRemovalReason
	for i, entry := range built {
		state := commitstatus.CommitStatusSuccess
		if s.rule.EnableStatusCheck || len(required) > 0 {
			statuses, err := pull_service.GetLatestPullCommitStatuses(ctx, s.repo.ID, entry.GroupHeadSHA)
			if err != nil {
				return nil, err
			}
			state = pull_service.MergeRequiredContextsCommitStatus(statuses, required)
		}
		nextState := pull_model.MergeQueueStateAwaitingChecks
		var reason issues_model.MergeQueueRemovalReason
		canAdvance := headGreen || i == ready
		switch {
		case state.IsSuccess():
			nextState = pull_model.MergeQueueStateMergeable
			if canAdvance && (!headGreen || i < cfg.MaxEntriesToMerge) {
				ready = i + 1
			}
		case state.IsFailure() || state.IsError():
			reason = issues_model.MergeQueueRemovedChecksFailed
		case entry.ChecksStartedUnix.AddDuration(timeout) <= timeutil.TimeStampNow():
			reason = issues_model.MergeQueueRemovedChecksTimeout
		}
		if reason != "" {
			nextState = pull_model.MergeQueueStateUnmergeable
			if failureReason == "" && canAdvance {
				failedAt, failureReason = i, reason
			}
		} else if headGreen && i < cfg.MaxEntriesToMerge {
			failureReason = "" // with HEADGREEN, a later passing entry covers an earlier failure
		}
		if entry.State != nextState {
			entry.State = nextState
			if err := pull_model.UpdateMergeQueueEntry(ctx, entry.MergeQueueEntry); err != nil {
				return nil, err
			}
		}
	}
	group := built[:min(ready, cfg.MaxEntriesToMerge)]
	if failureReason != "" {
		return group, s.dropEntry(ctx, failedAt, failureReason)
	}
	return group, nil
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

func (s *queueState) mergeReadyEntries(ctx context.Context, base string) (bool, error) {
	group, err := s.readyGroup(ctx, base)
	if err != nil || len(group) == 0 {
		return false, err
	}
	cfg := s.rule.MergeQueue
	cutoff := timeutil.TimeStampNow().AddDuration(-time.Duration(cfg.MinEntriesToMergeWaitMinutes) * time.Minute)
	if len(group) < cfg.MinEntriesToMerge && !slices.ContainsFunc(group, func(entry *QueueEntry) bool {
		return entry.CreatedUnix <= cutoff
	}) {
		return false, nil
	}
	for i, entry := range group {
		if blocked, err := entry.IsBlocked(ctx, s.gitRepo, s.rule); err != nil {
			return false, err
		} else if blocked {
			return false, s.dropEntry(ctx, i, issues_model.MergeQueueRemovedNotMergeable)
		}
	}
	if err := pull_service.MergeQueuePullRequests(ctx, group); err != nil {
		if rejected, ok := errors.AsType[*git.ErrPushRejected](err); ok && !strings.Contains(rejected.Message, private.InternalServerConnectionError) {
			last := len(group) - 1
			log.Warn("Merge queue push of %-v rejected: %v", group[last].PullRequest, err)
			return false, s.dropEntry(ctx, last, issues_model.MergeQueueRemovedNotMergeable)
		}
		return false, err
	}
	for _, entry := range group {
		if !entry.PullRequest.HasMerged {
			continue
		}
		if err := s.removeEntries(ctx, issues_model.MergeQueueRemovedMerged, entry); err != nil {
			return true, err
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
			reason, err := entry.CreateMergeQueueCommit(ctx, s.gitRepo, s.rule, base)
			if err != nil {
				return err
			} else if reason != "" {
				if err := s.removeEntries(ctx, reason, entry); err != nil {
					return err
				}
				addToQueue(s.repo.ID, s.branch)
				continue
			}
			entry.State, entry.ChecksStartedUnix = pull_model.MergeQueueStateAwaitingChecks, timeutil.TimeStampNow()
			if err := pull_model.UpdateMergeQueueEntry(ctx, entry.MergeQueueEntry); err != nil {
				return err
			}
			notify_service.MergeGroupChanged(ctx, entry.Enqueuer, s.repo, entry.MergeQueueEntry, api.HookMergeGroupChecksRequested, "")
			scheduled = true
		}
		if entry.State == pull_model.MergeQueueStateAwaitingChecks { // passed entries free their build slot while waiting for the minimum group size
			building++
		}
		base = entry.GroupHeadSHA
	}
	if scheduled {
		addToQueue(s.repo.ID, s.branch)
	}
	return nil
}

func destroyGroup(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, entry *pull_model.MergeQueueEntry, reason string) {
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
		if err != nil || !deleted || pr == nil || reason == issues_model.MergeQueueRemovedMerged || reason == issues_model.MergeQueueRemovedAlreadyMerged {
			return deleted, err
		}
		_, err = issues_model.CreatePullRequestComment(ctx, issues_model.CommentTypePRRemovedFromMergeQueue, pr, doer, string(reason))
		return true, err
	})
	if err != nil || !deleted {
		return err
	}
	if entry.IsBuilt() {
		destroyGroup(ctx, repo, doer, entry, util.Iif(reason == issues_model.MergeQueueRemovedMerged, "merged", "dequeued"))
	}
	if pr != nil {
		notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, api.HookIssueDequeued, reason)
	}
	if reason == issues_model.MergeQueueRemovedMerged {
		repo_service.DeleteBranchAfterMergeIfNeeded(ctx, doer, repo, pr, nil)
	}
	return nil
}
