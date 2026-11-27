// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"strings"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"
)

func MergesThroughQueue(ctx context.Context, pb *git_model.ProtectedBranch, repo *repo_model.Repository, doer *user_model.User, force bool) bool {
	return pb != nil && pb.RequireMergeQueue && !(force && git_model.CanBypassBranchProtection(ctx, pb, doer, access_model.IsUserRepoAdmin(ctx, repo, doer)))
}

type MergeQueueEntry struct {
	*pull_model.MergeQueueEntry
	Position    int
	PullRequest *issues_model.PullRequest
	Enqueuer    *user_model.User
}

func (entry *MergeQueueEntry) RestoreInterruptedMerge(ctx context.Context) error {
	pr := entry.PullRequest
	if pr == nil || pr.HasMerged || pr.Issue.IsClosed || pr.MergedCommitID == "" {
		return nil
	}
	return globallock.LockAndDo(ctx, getPullWorkingLockKey(entry.PullID), func(ctx context.Context) error {
		pr, err := issues_model.GetPullRequestByID(ctx, entry.PullID)
		if err != nil {
			return err
		}
		if !pr.HasMerged {
			if merged, err := restoreInterruptedMerge(ctx, pr); err != nil || !merged {
				return err
			}
		}
		entry.PullRequest = pr
		return pr.LoadIssue(ctx)
	})
}

func (entry *MergeQueueEntry) IsBlocked(ctx context.Context, gitRepo *git.Repository, rule *git_model.ProtectedBranch) (bool, error) {
	if entry.Enqueuer.IsGhost() {
		return true, nil
	}
	pr := entry.PullRequest
	perm, err := access_model.GetIndividualUserRepoPermission(ctx, pr.BaseRepo, entry.Enqueuer)
	if err != nil {
		return false, err
	}
	if !git_model.IsUserMergeWhitelisted(ctx, rule, entry.Enqueuer.ID, perm) {
		return true, nil
	}
	if noDeps, err := issues_model.IssueNoDependenciesLeft(ctx, pr.Issue); err != nil || !noDeps {
		return !noDeps, err
	}
	if pr.IsWorkInProgress(ctx) || checkPullReviews(ctx, rule, pr) != nil || !issue_service.HasAllRequiredCodeownerReviews(ctx, rule, pr) {
		return true, nil
	}
	if !entry.IsBuilt() {
		return false, nil
	}
	_, err = CheckFileProtection(ctx, gitRepo, pr.BaseBranch, entry.BaseSHA, entry.GroupHeadSHA, rule.GetProtectedFilePatterns(), 1, nil) // the final push skips this check when the last enqueuer can bypass protection
	if IsErrFilePathProtected(err) {
		return true, nil
	}
	return false, err
}

func (entry *MergeQueueEntry) CreateMergeQueueCommit(ctx context.Context, gitRepo *git.Repository, rule *git_model.ProtectedBranch, base string) (issues_model.MergeQueueRemovalReason, error) {
	if blocked, err := entry.IsBlocked(ctx, gitRepo, rule); err != nil {
		return "", err
	} else if blocked {
		return issues_model.MergeQueueRemovedNotMergeable, nil
	}
	pr, mergeStyle := entry.PullRequest, rule.MergeQueue.MergeMethod
	message, body, err := GetDefaultMergeMessage(ctx, gitRepo, pr, mergeStyle)
	if err != nil {
		return "", err
	}
	if mergeStyle == repo_model.MergeStyleSquash {
		squashCommitMessages, err := GetSquashMergeCommitMessages(ctx, pr)
		if err != nil {
			return "", err
		}
		body = git.CommitMessageMerge(squashCommitMessages, body)
	}
	if body != "" {
		message += "\n\n" + body
	}

	mergeCtx, cancel, err := createTemporaryRepoForMerge(ctx, pr, entry.Enqueuer, entry.HeadSHA, base)
	if IsErrSHADoesNotMatch(err) {
		return issues_model.MergeQueueRemovedHeadChanged, nil
	} else if err != nil {
		return "", err
	}
	defer cancel()
	if err := doMergeStyle(mergeCtx, mergeStyle, message); err != nil {
		switch {
		case IsErrMergeConflicts(err) || IsErrRebaseConflicts(err) || IsErrMergeUnrelatedHistories(err):
			return issues_model.MergeQueueRemovedConflict, nil
		case gitcmd.IsErrorExitCode(err, 1) || errors.Is(err, util.ErrInvalidArgument):
			log.Error("CreateMergeQueueCommit %-v: %v", pr, err)
			return issues_model.MergeQueueRemovedNotMergeable, nil
		}
		return "", err
	}
	commitID, err := git.GetFullCommitID(ctx, mergeCtx.tmpRepo, tmpRepoBaseBranch)
	if err != nil {
		return "", fmt.Errorf("CreateMergeQueueCommit: %w", err)
	}
	if setting.LFS.StartServer {
		if err := LFSPush(ctx, mergeCtx.tmpRepo, commitID, base, pr); err != nil {
			return "", err
		}
	}
	ref := pull_model.MergeQueueGroupRef(pr.BaseBranch, pr.Index, base)
	if err := git.FetchRemoteCommitUpdateRef(ctx, pr.BaseRepo, mergeCtx.tmpRepo, git.BranchPrefix+tmpRepoBaseBranch, ref); err != nil {
		return "", fmt.Errorf("CreateMergeQueueCommit: fetch into %s: %w", ref, err)
	}
	entry.BaseSHA, entry.GroupHeadSHA, entry.GroupRef = base, commitID, ref
	return "", nil
}

func MergeQueueRequiredContexts(required []string) []string {
	adapted := make([]string, len(required))
	for i, name := range required {
		if prefix, event, _ := strings.CutLast(name, " "); event == "(pull_request)" || event == "(pull_request_target)" || event == "(push)" {
			name = prefix + " (merge_group)"
		}
		adapted[i] = name
	}
	return adapted
}

func MergeQueuePullRequests(ctx context.Context, entries []*MergeQueueEntry) (err error) {
	last := entries[len(entries)-1]
	repo, baseBranch := last.PullRequest.BaseRepo, last.PullRequest.BaseBranch
	for _, entry := range entries {
		release, err := globallock.Lock(ctx, getPullWorkingLockKey(entry.PullID))
		if err != nil {
			return err
		}
		defer release()
	}
	for _, entry := range entries {
		pr, err := issues_model.GetPullRequestByID(ctx, entry.PullID)
		if err != nil {
			return err
		}
		if err := pr.LoadIssue(ctx); err != nil {
			return err
		}
		if pr.HasMerged || pr.Issue.IsClosed {
			return fmt.Errorf("MergeQueuePullRequests: %-v was merged or closed", pr)
		}
		if pr.BaseBranch != baseBranch {
			return fmt.Errorf("MergeQueuePullRequests: %-v no longer targets %s", pr, baseBranch)
		}
		headSHA, err := git.GetFullCommitID(ctx, repo, pr.GetGitHeadRefName())
		if err != nil {
			return err
		} else if headSHA != entry.HeadSHA {
			return ErrSHADoesNotMatch{GivenSHA: entry.HeadSHA, CurrentSHA: headSHA}
		}
	}
	defer func() {
		if err == nil {
			return
		}
		for _, entry := range entries {
			clearUnmergedIntent(ctx, entry.PullRequest)
		}
	}()
	for _, entry := range entries {
		if err := recordMergeIntent(ctx, entry.PullRequest, entry.Enqueuer, entry.GroupHeadSHA, false); err != nil {
			return fmt.Errorf("MergeQueuePullRequests: %w", err)
		}
	}
	baseRef := git.BranchPrefix + baseBranch
	if err := git.PushManaged(ctx, repo, repo, git.PushOptions{
		Branch:         last.GroupRef + ":" + baseRef, // git resolves a bare SHA to a same-named branch first, the lease rejects a base rewound since the build
		ForceWithLease: baseRef + ":" + entries[0].BaseSHA,
		Env:            repo_module.FullPushingEnvironment(last.Enqueuer, last.Enqueuer, repo, repo.Name, last.PullID, last.PullRequest.Index),
	}); err != nil {
		if rejected, ok := errors.AsType[*git.ErrPushRejected](err); ok && (strings.Contains(rejected.StdErr, "(stale info)") || strings.Contains(rejected.StdErr, "incorrect old value provided")) {
			err = &git.ErrPushOutOfDate{StdOut: rejected.StdOut, StdErr: rejected.StdErr, Err: rejected.Err}
		}
		return fmt.Errorf("MergeQueuePullRequests: push to %s: %w", baseBranch, err)
	}

	for _, entry := range entries {
		pr := entry.PullRequest
		pr.HasMerged, err = MarkAsMerged(ctx, pr, pr.MergedCommitID, pr.MergedUnix, entry.Enqueuer, pr.Status)
		if err != nil {
			log.Error("MarkAsMerged %-v: %v", pr, err)
		}
	}
	addTestPullRequestTaskAfterWebOperation(last.PullRequest, last.Enqueuer)
	return nil
}
