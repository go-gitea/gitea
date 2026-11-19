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
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
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

// Reasons of the merge_group destroyed event, named like GitHub's
const (
	destroyedMerged      = "merged"
	destroyedInvalidated = "invalidated"
	destroyedDequeued    = "dequeued"
)

var branchQueue *queue.WorkerPoolQueue[string]

// Init leaves existing queues to the cron task, the server can't answer the git hooks of a merge yet
func Init() error {
	notify_service.RegisterNotifier(&notifier{})
	branchQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "merge_queue", handleBranches)
	if branchQueue == nil {
		return errors.New("unable to create merge_queue queue")
	}
	go graceful.GetManager().RunWithCancel(branchQueue)
	return nil
}

// CheckAllQueues schedules every non-empty queue to resume after a restart and to enforce timeouts and wait times
func CheckAllQueues(ctx context.Context) error {
	branches, err := pull_model.GetMergeQueueBranches(ctx)
	if err != nil {
		return err
	}
	for _, b := range branches {
		addToQueue(b.RepoID, b.BaseBranch)
	}
	return nil
}

// addToQueue never blocks the caller: with an immediate queue the merge's own push hooks would otherwise wait for the branch lock held by the merge
func addToQueue(repoID int64, branch string) {
	go func() {
		if err := branchQueue.Push(fmt.Sprintf("%d:%s", repoID, branch)); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
			log.Error("Unable to push merge queue of repo %d branch %s: %v", repoID, branch, err)
		}
	}()
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
	repo     *repo_model.Repository
	gitRepo  *git.Repository
	pb       *git_model.ProtectedBranch
	cfg      git_model.MergeQueueConfig
	required []string // required contexts adapted to queue commits
	branch   string
	entries  []*pull_model.MergeQueueEntry
	prs      map[int64]*issues_model.PullRequest
	users    map[int64]*user_model.User
}

// processQueue advances the queue by one step, the events it causes schedule the next
func processQueue(ctx context.Context, repoID int64, branch string) error {
	entries, err := pull_model.GetMergeQueueEntries(ctx, repoID, branch)
	if err != nil || len(entries) == 0 {
		return err
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repoID, branch)
	if err != nil {
		return err
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return err
	}
	defer closer.Close()

	prs, err := loadPulls(ctx, entries)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		pr.BaseRepo = repo
	}
	users, err := loadEnqueuers(ctx, entries)
	if err != nil {
		return err
	}
	s := &queueState{repo: repo, gitRepo: gitRepo, pb: pb, branch: branch, prs: prs, users: users}
	if pb == nil || !pb.RequireMergeQueue {
		return s.removeEntries(ctx, entries, issues_model.MergeQueueRemovedNotMergeable)
	}
	s.cfg = pb.MergeQueue
	required, err := pull_service.EffectiveRequiredContexts(ctx, repo, pb)
	if err != nil {
		return err
	}
	s.required = pull_service.MergeQueueRequiredContexts(required)

	if err := s.dropStaleEntries(ctx, entries); err != nil {
		return err
	}
	baseSHA, err := gitRepo.GetBranchCommitID(ctx, branch)
	if git.IsErrNotExist(err) {
		return s.removeEntries(ctx, s.entries, issues_model.MergeQueueRemovedBaseDeleted)
	} else if err != nil {
		return err
	}
	if err := s.invalidateBrokenChain(ctx, baseSHA); err != nil {
		return err
	}
	if err := s.evaluateChecks(ctx); err != nil {
		return err
	}
	if merged, err := s.mergeReadyEntries(ctx); err != nil || merged {
		// the push to the base branch schedules the next step
		return err
	}
	return s.buildEntries(ctx, baseSHA)
}

func (s *queueState) enqueuer(e *pull_model.MergeQueueEntry) *user_model.User {
	return user_model.GetPossibleUserFromMap(e.EnqueuerID, s.users)
}

// dropStaleEntries removes closed, merged and retargeted pull requests and those with new commits, as GitHub does
func (s *queueState) dropStaleEntries(ctx context.Context, entries []*pull_model.MergeQueueEntry) error {
	for _, e := range entries {
		pr := s.prs[e.PullID]
		if pr == nil || pr.HasMerged || pr.Issue.IsClosed || pr.BaseBranch != s.branch {
			reason := util.Iif(pr != nil && pr.BaseBranch != s.branch, issues_model.MergeQueueRemovedBaseChanged, issues_model.MergeQueueRemovedClosed)
			if err := s.removeEntry(ctx, e, reason); err != nil {
				return err
			}
			continue
		}
		headSHA, err := s.gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
		if err != nil {
			return err
		}
		if headSHA != e.HeadSHA {
			if err := s.removeEntry(ctx, e, issues_model.MergeQueueRemovedHeadChanged); err != nil {
				return err
			}
			continue
		}
		s.entries = append(s.entries, e)
	}
	return nil
}

// isBlocked re-checks what can change without a new commit: access of the enqueuer, who the merge is pushed as, title, dependencies and reviews
func (s *queueState) isBlocked(ctx context.Context, e *pull_model.MergeQueueEntry, pr *issues_model.PullRequest) (bool, error) {
	doer := s.enqueuer(e)
	if doer.IsGhost() {
		return true, nil
	}
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, s.repo, doer)
	if err != nil {
		return false, err
	}
	if !git_model.IsUserMergeWhitelisted(ctx, s.pb, doer.ID, perm) {
		return true, nil
	}
	if noDeps, err := issues_model.IssueNoDependenciesLeft(ctx, pr.Issue); err != nil || !noDeps {
		return !noDeps, err
	}
	return pr.IsWorkInProgress(ctx) || pull_service.CheckPullReviews(ctx, s.pb, pr) != nil, nil
}

// invalidateBrokenChain discards queue commits that are no longer built on the base branch and the entries before them
func (s *queueState) invalidateBrokenChain(ctx context.Context, baseSHA string) error {
	expected := baseSHA
	broken := false
	for _, e := range s.entries {
		if !e.IsBuilt() {
			broken = true // later entries must be built on this one first
			continue
		}
		if broken || e.BaseSHA != expected {
			broken = true
			if err := s.invalidateEntry(ctx, e); err != nil {
				return err
			}
			continue
		}
		expected = e.GroupHeadSHA
	}
	return nil
}

func (s *queueState) evaluateChecks(ctx context.Context) error {
	timeout := time.Duration(s.cfg.CheckResponseTimeoutMinutes) * time.Minute
	timedOut := map[int64]bool{} // a HEADGREEN entry kept to a later run is reported as failed
	for _, e := range s.entries {
		if e.State != pull_model.MergeQueueStateAwaitingChecks {
			continue
		}
		state := commitstatus.CommitStatusSuccess // a rule created through the API may require no contexts
		if len(s.required) > 0 {
			statuses, err := pull_service.GetLatestPullCommitStatuses(ctx, s.repo.ID, e.GroupHeadSHA)
			if err != nil {
				return err
			}
			state = pull_service.MergeRequiredContextsCommitStatus(statuses, s.required)
		}
		switch {
		case state.IsSuccess():
			e.State = pull_model.MergeQueueStateMergeable
		case state.IsFailure() || state.IsError():
			e.State = pull_model.MergeQueueStateUnmergeable
		case e.ChecksStartedUnix.AddDuration(timeout) <= timeutil.TimeStampNow():
			e.State = pull_model.MergeQueueStateUnmergeable
			timedOut[e.ID] = true
		default:
			continue
		}
		if err := pull_model.UpdateMergeQueueEntry(ctx, e); err != nil {
			return err
		}
	}
	return s.removeFailedEntries(ctx, timedOut)
}

// removeFailedEntries drops failed entries and rebuilds the ones after them.
// With HEADGREEN a failed entry stays while a later queue commit, which contains its changes, may still pass.
func (s *queueState) removeFailedEntries(ctx context.Context, timedOut map[int64]bool) error {
	headGreen := s.cfg.GroupingStrategy == git_model.MergeQueueGroupingHeadGreen
	for i := 0; i < len(s.entries); i++ {
		e := s.entries[i]
		if e.State != pull_model.MergeQueueStateUnmergeable {
			continue
		}
		if headGreen && s.hasPendingOrPassingAfter(i) {
			continue
		}
		if err := s.dropEntry(ctx, i, util.Iif(timedOut[e.ID], issues_model.MergeQueueRemovedChecksTimeout, issues_model.MergeQueueRemovedChecksFailed)); err != nil {
			return err
		}
		i--
	}
	return nil
}

// dropEntry removes the entry at index i, the queue commits after it contain its changes and are rebuilt
func (s *queueState) dropEntry(ctx context.Context, i int, reason issues_model.MergeQueueRemovalReason) error {
	if err := s.removeEntry(ctx, s.entries[i], reason); err != nil {
		return err
	}
	for _, next := range s.entries[i+1:] {
		if next.IsBuilt() {
			if err := s.invalidateEntry(ctx, next); err != nil {
				return err
			}
		}
	}
	s.entries = slices.Delete(s.entries, i, i+1)
	addToQueue(s.repo.ID, s.branch) // the entries before it may now merge without waiting for the cron task
	return nil
}

// hasPendingOrPassingAfter only looks within the merge group size, later entries can't merge the failed one
func (s *queueState) hasPendingOrPassingAfter(i int) bool {
	end := min(len(s.entries), s.cfg.MaxEntriesToMerge)
	for _, e := range s.entries[min(i+1, end):end] {
		if e.State == pull_model.MergeQueueStateAwaitingChecks || e.State == pull_model.MergeQueueStateMergeable {
			return true
		}
	}
	return false
}

// mergeReadyEntries fast-forwards the base branch to the furthest passing queue commit allowed by the group size settings
func (s *queueState) mergeReadyEntries(ctx context.Context) (bool, error) {
	last := -1
	for i, e := range s.entries {
		if i >= s.cfg.MaxEntriesToMerge || !e.IsBuilt() {
			break
		}
		if e.State == pull_model.MergeQueueStateMergeable {
			last = i
		} else if s.cfg.GroupingStrategy != git_model.MergeQueueGroupingHeadGreen {
			break
		}
	}
	if last < 0 {
		return false, nil
	}
	waited := s.entries[0].CreatedUnix.AddDuration(time.Duration(s.cfg.MinEntriesToMergeWaitMinutes)*time.Minute) <= timeutil.TimeStampNow()
	if last+1 < s.cfg.MinEntriesToMerge && !waited {
		return false, nil
	}

	group := s.entries[:last+1]
	items := make([]pull_service.MergeQueueItem, 0, len(group))
	for i, e := range group {
		pr := s.prs[e.PullID]
		if blocked, err := s.isBlocked(ctx, e, pr); err != nil {
			return false, err
		} else if blocked {
			// the queue commits are rebuilt without it in this run
			return false, s.dropEntry(ctx, i, issues_model.MergeQueueRemovedNotMergeable)
		}
		items = append(items, pull_service.MergeQueueItem{PR: pr, Doer: s.enqueuer(e), CommitID: e.GroupHeadSHA})
	}
	if err := pull_service.MergeQueuePullRequests(ctx, s.repo, s.branch, items); err != nil {
		if _, ok := errors.AsType[*git.ErrPushRejected](err); ok {
			// a rejection, unlike an out-of-date push, would repeat on every retry
			log.Warn("Merge queue push of %-v rejected: %v", items[last].PR, err)
			return false, s.dropEntry(ctx, last, issues_model.MergeQueueRemovedNotMergeable)
		}
		return false, err
	}

	for i, e := range group {
		if _, err := pull_model.DeleteMergeQueueEntry(ctx, e.PullID); err != nil {
			log.Error("DeleteMergeQueueEntry: %v", err)
		}
		destroyGroup(ctx, s.repo, items[i].Doer, e, destroyedMerged)
		if shouldDelete, err := pull_service.ShouldDeleteBranchAfterMerge(ctx, nil, s.repo, items[i].PR); err != nil {
			log.Error("ShouldDeleteBranchAfterMerge: %v", err)
		} else if shouldDelete {
			if err := repo_service.DeleteBranchAfterMerge(ctx, items[i].Doer, items[i].PR.ID, nil); err != nil {
				log.Error("DeleteBranchAfterMerge: %v", err)
			}
		}
	}
	s.entries = s.entries[last+1:]
	return true, nil
}

func (s *queueState) buildEntries(ctx context.Context, base string) error {
	building, scheduled := 0, false
	for i := 0; i < len(s.entries); i++ {
		e := s.entries[i]
		if e.IsBuilt() {
			if e.State == pull_model.MergeQueueStateAwaitingChecks {
				building++
			}
			base = e.GroupHeadSHA
			continue
		}
		if building >= s.cfg.MaxEntriesToBuild {
			break
		}
		pr, doer := s.prs[e.PullID], s.enqueuer(e)
		ref := pull_model.MergeQueueGroupRef(s.branch, pr.Index, e.HeadSHA)
		blocked, err := s.isBlocked(ctx, e, pr)
		if err != nil {
			return err
		}
		var sha string
		var reason issues_model.MergeQueueRemovalReason
		if blocked {
			reason = issues_model.MergeQueueRemovedNotMergeable
		} else if sha, err = pull_service.CreateMergeQueueCommit(ctx, pr, doer, s.gitRepo, s.cfg.MergeMethod, base, e.HeadSHA, ref); err != nil {
			reason = buildFailureReason(pr, err)
		}
		if reason != "" {
			// an entry that can't be built would otherwise block the queue
			if err := s.dropEntry(ctx, i, reason); err != nil {
				return err
			}
			i--
			continue
		}
		e.State = pull_model.MergeQueueStateAwaitingChecks
		e.BaseSHA, e.GroupHeadSHA, e.GroupRef = base, sha, ref
		e.ChecksStartedUnix = timeutil.TimeStampNow()
		if err := pull_model.UpdateMergeQueueEntry(ctx, e); err != nil {
			return err
		}
		notify_service.MergeGroupChanged(ctx, doer, s.repo, e, api.HookMergeGroupChecksRequested, "")
		building++
		scheduled = true
		base = sha
	}
	if scheduled {
		// statuses may already exist or no checks may be required
		addToQueue(s.repo.ID, s.branch)
	}
	return nil
}

func buildFailureReason(pr *issues_model.PullRequest, err error) issues_model.MergeQueueRemovalReason {
	switch {
	case pull_service.IsErrSHADoesNotMatch(err):
		return issues_model.MergeQueueRemovedHeadChanged
	case pull_service.IsErrMergeConflicts(err) || pull_service.IsErrRebaseConflicts(err) || pull_service.IsErrMergeUnrelatedHistories(err):
		return issues_model.MergeQueueRemovedConflict
	}
	log.Error("CreateMergeQueueCommit %-v: %v", pr, err)
	return issues_model.MergeQueueRemovedNotMergeable
}

// the entry is built again on the new base
func (s *queueState) invalidateEntry(ctx context.Context, e *pull_model.MergeQueueEntry) error {
	destroyGroup(ctx, s.repo, s.enqueuer(e), e, destroyedInvalidated)
	e.State = pull_model.MergeQueueStateQueued
	e.BaseSHA, e.GroupHeadSHA, e.GroupRef, e.ChecksStartedUnix = "", "", "", 0
	return pull_model.UpdateMergeQueueEntry(ctx, e)
}

func destroyGroup(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, e *pull_model.MergeQueueEntry, reason string) {
	if !e.IsBuilt() {
		return
	}
	if err := git.RemoveRef(ctx, repo, e.GroupRef); err != nil {
		log.Error("RemoveRef %s: %v", e.GroupRef, err)
	}
	notify_service.MergeGroupChanged(ctx, doer, repo, e, api.HookMergeGroupDestroyed, reason)
}

func (s *queueState) removeEntries(ctx context.Context, entries []*pull_model.MergeQueueEntry, reason issues_model.MergeQueueRemovalReason) error {
	for _, e := range entries {
		if err := s.removeEntry(ctx, e, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s *queueState) removeEntry(ctx context.Context, e *pull_model.MergeQueueEntry, reason issues_model.MergeQueueRemovalReason) error {
	return removeEntry(ctx, s.repo, e, s.prs[e.PullID], s.enqueuer(e), reason)
}

// pr is nil when the pull request was deleted
func removeEntry(ctx context.Context, repo *repo_model.Repository, e *pull_model.MergeQueueEntry, pr *issues_model.PullRequest, doer *user_model.User, reason issues_model.MergeQueueRemovalReason) error {
	// the comment is written in the same transaction, so it exists once the entry is gone
	var deleted bool
	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		if deleted, err = pull_model.DeleteMergeQueueEntry(ctx, e.PullID); err != nil || !deleted || reason == issues_model.MergeQueueRemovedClosed || pr == nil {
			return err
		}
		return issues_model.CreateMergeQueueComment(ctx, pr, doer, reason)
	})
	if err != nil || !deleted {
		return err
	}
	destroyGroup(ctx, repo, doer, e, destroyedDequeued)
	if pr != nil {
		notify_service.PullRequestMergeQueueChanged(ctx, doer, pr, false, string(reason))
	}
	return nil
}
