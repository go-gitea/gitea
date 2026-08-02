// Copyright 2021 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	"gitea.dev/services/automergequeue"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
	repo_service "gitea.dev/services/repository"
)

// Init runs the task queue to that handles auto merges
func Init() error {
	notify_service.RegisterNotifier(NewNotifier())

	automergequeue.AutoMergeQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "pr_auto_merge", handler)
	if automergequeue.AutoMergeQueue == nil {
		return errors.New("unable to create pr_auto_merge queue")
	}
	go graceful.GetManager().RunWithCancel(automergequeue.AutoMergeQueue)
	return nil
}

// maxTransientRetries bounds how many times a queue item is requeued after a
// transient failure (e.g. a temporary database or filesystem error), so a
// persistently failing item cannot keep a queue worker spinning forever.
// When the bound is reached the scheduled merge stays recorded in the
// database, but it is only evaluated again when a new event arrives for the
// pull request.
const maxTransientRetries = 30

var (
	transientRetryMu     sync.Mutex
	transientRetryCounts = map[string]int{}
)

// countTransientFailure records a transient failure for the given queue item
// and reports whether the item should be requeued for another attempt.
func countTransientFailure(item string) (requeue bool) {
	transientRetryMu.Lock()
	defer transientRetryMu.Unlock()
	transientRetryCounts[item]++
	if transientRetryCounts[item] >= maxTransientRetries {
		delete(transientRetryCounts, item)
		return false
	}
	return true
}

func clearTransientFailures(item string) {
	transientRetryMu.Lock()
	defer transientRetryMu.Unlock()
	delete(transientRetryCounts, item)
}

// handle passed PR IDs and test the PRs. Items that fail transiently are
// returned to the queue so its retry mechanism can requeue them; before this,
// any transient failure permanently dropped the scheduled merge, leaving the
// pull request green and armed but never merged.
func handler(items ...string) []string {
	var unhandled []string
	for _, s := range items {
		var id int64
		var sha string
		if _, err := fmt.Sscanf(s, "%d_%s", &id, &sha); err != nil {
			log.Error("could not parse data from pr_auto_merge queue (%v): %v", s, err)
			continue
		}
		if err := handlePullRequestAutoMerge(id, sha); err != nil {
			if countTransientFailure(s) {
				log.Warn("PullRequest[%d] automerge evaluation failed, it will be retried: %v", id, err)
				unhandled = append(unhandled, s)
			} else {
				log.Error("PullRequest[%d] automerge evaluation failed %d times, giving up until a new event for it arrives: %v", id, maxTransientRetries, err)
			}
			continue
		}
		clearTransientFailures(s)
	}
	return unhandled
}

// ScheduleAutoMerge if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest, style repo_model.MergeStyle, message string, deleteBranchAfterMerge bool) (scheduled bool, err error) {
	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := pull_model.ScheduleAutoMerge(ctx, doer, pull.ID, style, message, deleteBranchAfterMerge); err != nil {
			return err
		}
		_, err = issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRScheduledToAutoMerge, pull, doer)
		return err
	})
	// Old code made "scheduled" to be true after "ScheduleAutoMerge", but it's not right:
	// If the transaction rolls back, then the pull request is not scheduled to auto merge.
	// So we should only set "scheduled" to true if there is no error.
	scheduled = err == nil
	if scheduled {
		log.Trace("Pull request [%d] scheduled for auto merge with style [%s] and message [%s]", pull.ID, style, message)
		automergequeue.StartPRCheckAndAutoMerge(ctx, pull)
	}
	return scheduled, err
}

// ReconcileScheduledAutoMerges re-queues an evaluation for every pull request
// that is still scheduled to auto merge.
//
// The queue is the only thing that drives a scheduled merge forward, and an
// evaluation can be lost: handlePullRequestAutoMerge returns on any error with
// nothing but a log line, and the handler reports every item as handled, so
// the queue never retries it. Once the head commit's checks have settled there
// is no further status event to enqueue a replacement, and the pull request
// stays green, scheduled and unmerged indefinitely.
//
// pull_auto_merge is the durable record of intent, so walking it rebuilds
// whatever queue state was lost — whichever way it was lost, including a
// restart discarding an in-memory queue. This is deliberately a sweep rather
// than a retry of the specific failure: it needs no attempt accounting and
// cannot spin, because the queue is unique-keyed and an evaluation for a pull
// request that is not ready returns immediately.
func ReconcileScheduledAutoMerges(ctx context.Context) error {
	return pull_model.IterateScheduledAutoMerges(ctx, func(ctx context.Context, am *pull_model.AutoMerge) error {
		pr, err := issues_model.GetPullRequestByID(ctx, am.PullID)
		if err != nil {
			// A scheduled row for a pull request that no longer exists is
			// stale, not a reason to abort the sweep.
			log.Error("ReconcileScheduledAutoMerges GetPullRequestByID[%d]: %v", am.PullID, err)
			return nil
		}
		if pr.HasMerged || !pr.IsStatusMergeable() {
			return nil
		}
		// StartPRCheckAndAutoMerge resolves the current head sha itself, so a
		// pull request whose head moved on is re-queued under its new key.
		automergequeue.StartPRCheckAndAutoMerge(ctx, pr)
		return nil
	})
}

// RemoveScheduledAutoMerge cancels a previously scheduled pull request
func RemoveScheduledAutoMerge(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := pull_model.DeleteScheduledAutoMerge(ctx, pull.ID); err != nil {
			return err
		}

		_, err := issues_model.CreateAutoMergeComment(ctx, issues_model.CommentTypePRUnScheduledToAutoMerge, pull, doer)
		return err
	})
}

// StartPRCheckAndAutoMergeBySHA start an automerge check and auto merge task for all pull requests of repository and SHA
func StartPRCheckAndAutoMergeBySHA(ctx context.Context, sha string, repo *repo_model.Repository) error {
	pulls, err := getPullRequestsByHeadSHA(ctx, sha, repo, func(pr *issues_model.PullRequest) bool {
		return !pr.HasMerged && pr.IsStatusMergeable()
	})
	if err != nil {
		return err
	}

	for _, pr := range pulls {
		automergequeue.AddToQueue(pr, sha)
	}

	return nil
}

func getPullRequestsByHeadSHA(ctx context.Context, sha string, repo *repo_model.Repository, filter func(*issues_model.PullRequest) bool) (map[int64]*issues_model.PullRequest, error) {
	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	refs, err := gitRepo.GetRefsBySha(ctx, sha, "")
	if err != nil {
		return nil, err
	}

	pulls := make(map[int64]*issues_model.PullRequest)

	for _, ref := range refs {
		// Each pull branch starts with refs/pull/ we then go from there to find the index of the pr and then
		// use that to get the pr.
		if strings.HasPrefix(ref, git.PullPrefix) {
			parts := strings.Split(ref[len(git.PullPrefix):], "/")

			// e.g. 'refs/pull/1/head' would be []string{"1", "head"}
			if len(parts) != 2 {
				log.Error("getPullRequestsByHeadSHA found broken pull ref [%s] on repo [%-v]", ref, repo)
				continue
			}

			prIndex, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				log.Error("getPullRequestsByHeadSHA found broken pull ref [%s] on repo [%-v]", ref, repo)
				continue
			}

			p, err := issues_model.GetPullRequestByIndex(ctx, repo.ID, prIndex)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if issues_model.IsErrPullRequestNotExist(err) {
					continue
				}
				return nil, err
			}

			if filter(p) {
				pulls[p.ID] = p
			}
		}
	}

	return pulls, nil
}

// isTerminalMergeCheckError reports whether a CheckPullMergeable failure is a
// definite refusal that retrying cannot cure, as opposed to a transient
// infrastructure error.
func isTerminalMergeCheckError(err error) bool {
	for _, terminal := range []error{
		pull_service.ErrHasMerged,
		pull_service.ErrIsClosed,
		pull_service.ErrNoPermissionToMerge,
		pull_service.ErrNotReadyToMerge,
		pull_service.ErrIsWorkInProgress,
		pull_service.ErrIsChecking,
		pull_service.ErrNotMergeableState,
		pull_service.ErrDependenciesLeft,
		pull_service.ErrHeadCommitsNotAllVerified,
	} {
		if errors.Is(err, terminal) {
			return true
		}
	}
	return false
}

// handlePullRequestAutoMerge is a function variable so tests can intercept it.
var handlePullRequestAutoMerge = realHandlePullRequestAutoMerge

// realHandlePullRequestAutoMerge merges the pull request if all checks are
// successful. It returns nil when the queue item is fully handled — the pull
// request was merged, or was refused for a reason retrying cannot cure — and
// an error for transient infrastructure failures (database reads, git
// repository access), so the caller can requeue the item instead of silently
// dropping the scheduled merge.
func realHandlePullRequestAutoMerge(pullID int64, sha string) error {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(),
		fmt.Sprintf("Handle AutoMerge of PR[%d] with sha[%s]", pullID, sha))
	defer finished()

	pr, err := issues_model.GetPullRequestByID(ctx, pullID)
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			log.Warn("GetPullRequestByID[%d]: pull request does not exist", pullID)
			return nil
		}
		return fmt.Errorf("GetPullRequestByID[%d]: %w", pullID, err)
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID)
	if err != nil {
		return fmt.Errorf("GetScheduledMergeByPullID[%d]: %w", pr.ID, err)
	}
	if !exists {
		return nil
	}

	if err = pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("LoadBaseRepo[%d]: %w", pr.ID, err)
	}

	// check the sha is the same as pull request head commit id
	baseGitRepo, err := git.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		return fmt.Errorf("OpenRepository[%d]: %w", pr.BaseRepoID, err)
	}
	defer baseGitRepo.Close()

	headCommitID, err := baseGitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		if git.IsErrNotExist(err) {
			log.Warn("Head ref of auto merge %-v does not exist: %v", pr, err)
			return nil
		}
		return fmt.Errorf("GetRefCommitID[%d]: %w", pr.ID, err)
	}
	if headCommitID != sha {
		log.Warn("Head commit id of auto merge %-v does not match sha [%s], it may means the head branch has been updated. Just ignore this request because a new request expected in the queue", pr, sha)
		return nil
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.
	if err = pr.LoadHeadRepo(ctx); err != nil {
		return fmt.Errorf("LoadHeadRepo[%d]: %w", pr.ID, err)
	}

	switch pr.Flow {
	case issues_model.PullRequestFlowGithub:
		headBranchExist := pr.HeadRepo != nil
		if headBranchExist {
			headBranchExist, _ = git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch)
		}
		if !headBranchExist {
			log.Warn("Head branch of auto merge %-v does not exist [HeadRepoID: %d, Branch: %s]", pr, pr.HeadRepoID, pr.HeadBranch)
			return nil
		}
	case issues_model.PullRequestFlowAGit:
		headBranchExist := git.IsReferenceExist(ctx, pr.BaseRepo, pr.GetGitHeadRefName())
		if !headBranchExist {
			log.Warn("Head branch of auto merge %-v does not exist [HeadRepoID: %d, Branch(Agit): %s]", pr, pr.HeadRepoID, pr.HeadBranch)
			return nil
		}
	default:
		log.Error("wrong flow type %d", pr.Flow)
		return nil
	}

	// Check if all checks succeeded
	pass, err := pull_service.IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		return fmt.Errorf("IsPullCommitStatusPass[%d]: %w", pr.ID, err)
	}
	if !pass {
		log.Info("Scheduled auto merge %-v has unsuccessful status checks", pr)
		return nil
	}

	// Merge if all checks succeeded
	doer, err := user_model.GetUserByID(ctx, scheduledPRM.DoerID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			log.Warn("Scheduled auto merge %-v: scheduling User[%d] no longer exists", pr, scheduledPRM.DoerID)
			return nil
		}
		return fmt.Errorf("GetUserByID[%d]: %w", scheduledPRM.DoerID, err)
	}

	perm, err := access_model.GetDoerRepoPermission(ctx, pr.BaseRepo, doer)
	if err != nil {
		return fmt.Errorf("GetDoerRepoPermission[%d]: %w", pr.BaseRepoID, err)
	}

	if err := pull_service.CheckPullMergeable(ctx, doer, &perm, pr, pull_service.MergeCheckTypeGeneral, scheduledPRM.MergeStyle, false); err != nil {
		if isTerminalMergeCheckError(err) {
			// the error carries the precise reason (e.g. for ErrNotReadyToMerge:
			// status checks, approvals, requested changes, official review
			// requests, behind base branch, protected files), so log it instead
			// of discarding it.
			log.Info("%-v is not ready for scheduled auto merge: %v", pr, err)
			return nil
		}
		return fmt.Errorf("CheckPullMergeable[%d]: %w", pr.ID, err)
	}

	if err := pull_service.Merge(ctx, pr, doer, scheduledPRM.MergeStyle, "", scheduledPRM.Message, true); err != nil {
		log.Error("pull_service.Merge: %v", err)
		// FIXME: if merge failed, we should display some error message to the pull request page.
		// The resolution is add a new column on automerge table named `error_message` to store the error message and displayed
		// on the pull request page. But this should not be finished in a bug fix PR which will be backport to release branch.
		return nil
	}

	deleteBranchAfterMerge, err := pull_service.ShouldDeleteBranchAfterMerge(ctx, &scheduledPRM.DeleteBranchAfterMerge, pr.BaseRepo, pr)
	if err != nil {
		log.Error("ShouldDeleteBranchAfterMerge: %v", err)
	} else if deleteBranchAfterMerge {
		if err = repo_service.DeleteBranchAfterMerge(ctx, doer, pr.ID, nil); err != nil {
			log.Error("DeleteBranchAfterMerge: %v", err)
		}
	}
	return nil
}
