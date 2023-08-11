// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/util"
	issue_service "code.gitea.io/gitea/services/issue"
)

// TODO: use clustered lock (unique queue? or *abuse* cache)
var pullWorkingPool = sync.NewExclusivePool()

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(ctx context.Context, repo *repo_model.Repository, issue *issues_model.Issue, labelIDs []int64, uuids []string, pr *issues_model.PullRequest, assigneeIDs []int64) error {
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		if !models.IsErrBranchDoesNotExist(err) {
			log.Error("CreateTemporaryRepoForPR %-v: %v", pr, err)
		}
		return err
	}
	defer cancel()

	if err := testPatch(ctx, prCtx, pr); err != nil {
		return err
	}

	divergence, err := git.GetDivergingCommits(ctx, prCtx.tmpBasePath, baseBranch, trackingBranch)
	if err != nil {
		return err
	}
	pr.CommitsAhead = divergence.Ahead
	pr.CommitsBehind = divergence.Behind

	assigneeCommentMap := make(map[int64]*issues_model.Comment)

	// add first push codes comment
	baseGitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		return err
	}
	defer baseGitRepo.Close()

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := issues_model.NewPullRequest(ctx, repo, issue, labelIDs, uuids, pr); err != nil {
			return err
		}

		for _, assigneeID := range assigneeIDs {
			comment, err := issue_service.AddAssigneeIfNotAssigned(ctx, issue, issue.Poster, assigneeID, false)
			if err != nil {
				return err
			}
			assigneeCommentMap[assigneeID] = comment
		}

		pr.Issue = issue
		issue.PullRequest = pr

		if pr.Flow == issues_model.PullRequestFlowGithub {
			err = PushToBaseRepo(ctx, pr)
		} else {
			err = UpdateRef(ctx, pr)
		}
		if err != nil {
			return err
		}

		compareInfo, err := baseGitRepo.GetCompareInfo(pr.BaseRepo.RepoPath(),
			git.BranchPrefix+pr.BaseBranch, pr.GetGitRefName(), false, false)
		if err != nil {
			return err
		}
		if len(compareInfo.Commits) == 0 {
			return nil
		}

		data := issues_model.PushActionContent{IsForcePush: false}
		data.CommitIDs = make([]string, 0, len(compareInfo.Commits))
		for i := len(compareInfo.Commits) - 1; i >= 0; i-- {
			data.CommitIDs = append(data.CommitIDs, compareInfo.Commits[i].ID.String())
		}

		dataJSON, err := json.Marshal(data)
		if err != nil {
			return err
		}

		ops := &issues_model.CreateCommentOptions{
			Type:        issues_model.CommentTypePullRequestPush,
			Doer:        issue.Poster,
			Repo:        repo,
			Issue:       pr.Issue,
			IsForcePush: false,
			Content:     string(dataJSON),
		}

		if _, err = issues_model.CreateComment(ctx, ops); err != nil {
			return err
		}

		return nil
	}); err != nil {
		// cleanup: this will only remove the reference, the real commit will be clean up when next GC
		if err1 := baseGitRepo.RemoveReference(pr.GetGitRefName()); err1 != nil {
			log.Error("RemoveReference: %v", err1)
		}
		return err
	}
	baseGitRepo.Close() // close immediately to avoid notifications will open the repository again

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, issue.Poster, issue.Content)
	if err != nil {
		return err
	}

	notification.NotifyNewPullRequest(ctx, pr, mentions)
	if len(issue.Labels) > 0 {
		notification.NotifyIssueChangeLabels(ctx, issue.Poster, issue, issue.Labels, nil)
	}
	if issue.Milestone != nil {
		notification.NotifyIssueChangeMilestone(ctx, issue.Poster, issue, 0)
	}
	if len(assigneeIDs) > 0 {
		for _, assigneeID := range assigneeIDs {
			assignee, err := user_model.GetUserByID(ctx, assigneeID)
			if err != nil {
				return ErrDependenciesLeft
			}
			notification.NotifyIssueChangeAssignee(ctx, issue.Poster, issue, assignee, false, assigneeCommentMap[assigneeID])
		}
	}

	return nil
}

// ChangeTargetBranch changes the target branch of this pull request, as the given user.
func ChangeTargetBranch(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, targetBranch string) (err error) {
	pullWorkingPool.CheckIn(fmt.Sprint(pr.ID))
	defer pullWorkingPool.CheckOut(fmt.Sprint(pr.ID))

	// Current target branch is already the same
	if pr.BaseBranch == targetBranch {
		return nil
	}

	if pr.Issue.IsClosed {
		return issues_model.ErrIssueIsClosed{
			ID:     pr.Issue.ID,
			RepoID: pr.Issue.RepoID,
			Index:  pr.Issue.Index,
		}
	}

	if pr.HasMerged {
		return models.ErrPullRequestHasMerged{
			ID:         pr.ID,
			IssueID:    pr.Index,
			HeadRepoID: pr.HeadRepoID,
			BaseRepoID: pr.BaseRepoID,
			HeadBranch: pr.HeadBranch,
			BaseBranch: pr.BaseBranch,
		}
	}

	// Check if branches are equal
	branchesEqual, err := IsHeadEqualWithBranch(ctx, pr, targetBranch)
	if err != nil {
		return err
	}
	if branchesEqual {
		return models.ErrBranchesEqual{
			HeadBranchName: pr.HeadBranch,
			BaseBranchName: targetBranch,
		}
	}

	// Check if pull request for the new target branch already exists
	existingPr, err := issues_model.GetUnmergedPullRequest(ctx, pr.HeadRepoID, pr.BaseRepoID, pr.HeadBranch, targetBranch, issues_model.PullRequestFlowGithub)
	if existingPr != nil {
		return issues_model.ErrPullRequestAlreadyExists{
			ID:         existingPr.ID,
			IssueID:    existingPr.Index,
			HeadRepoID: existingPr.HeadRepoID,
			BaseRepoID: existingPr.BaseRepoID,
			HeadBranch: existingPr.HeadBranch,
			BaseBranch: existingPr.BaseBranch,
		}
	}
	if err != nil && !issues_model.IsErrPullRequestNotExist(err) {
		return err
	}

	// Set new target branch
	oldBranch := pr.BaseBranch
	pr.BaseBranch = targetBranch

	// Refresh patch
	if err := TestPatch(pr); err != nil {
		return err
	}

	// Update target branch, PR diff and status
	// This is the same as checkAndUpdateStatus in check service, but also updates base_branch
	if pr.Status == issues_model.PullRequestStatusChecking {
		pr.Status = issues_model.PullRequestStatusMergeable
	}

	// Update Commit Divergence
	divergence, err := GetDiverging(ctx, pr)
	if err != nil {
		return err
	}
	pr.CommitsAhead = divergence.Ahead
	pr.CommitsBehind = divergence.Behind

	if err := pr.UpdateColsIfNotMerged(ctx, "merge_base", "status", "conflicted_files", "changed_protected_files", "base_branch", "commits_ahead", "commits_behind"); err != nil {
		return err
	}

	// Create comment
	options := &issues_model.CreateCommentOptions{
		Type:   issues_model.CommentTypeChangeTargetBranch,
		Doer:   doer,
		Repo:   pr.Issue.Repo,
		Issue:  pr.Issue,
		OldRef: oldBranch,
		NewRef: targetBranch,
	}
	if _, err = issue_service.CreateComment(ctx, options); err != nil {
		return fmt.Errorf("CreateChangeTargetBranchComment: %w", err)
	}

	return nil
}

func checkForInvalidation(ctx context.Context, requests issues_model.PullRequestList, repoID int64, doer *user_model.User, branch string) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByIDCtx: %w", err)
	}
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	go func() {
		// FIXME: graceful: We need to tell the manager we're doing something...
		err := InvalidateCodeComments(ctx, requests, doer, gitRepo, branch)
		if err != nil {
			log.Error("PullRequestList.InvalidateCodeComments: %v", err)
		}
		gitRepo.Close()
	}()
	return nil
}

// AddTestPullRequestTask adds new test tasks by given head/base repository and head/base branch,
// and generate new patch for testing as needed.
func AddTestPullRequestTask(doer *user_model.User, repoID int64, branch string, isSync bool, oldCommitID, newCommitID string) {
	log.Trace("AddTestPullRequestTask [head_repo_id: %d, head_branch: %s]: finding pull requests", repoID, branch)
	graceful.GetManager().RunWithShutdownContext(func(ctx context.Context) {
		// There is no sensible way to shut this down ":-("
		// If you don't let it run all the way then you will lose data
		// TODO: graceful: AddTestPullRequestTask needs to become a queue!

		// GetUnmergedPullRequestsByHeadInfo() only return open and unmerged PR.
		prs, err := issues_model.GetUnmergedPullRequestsByHeadInfo(repoID, branch)
		if err != nil {
			log.Error("Find pull requests [head_repo_id: %d, head_branch: %s]: %v", repoID, branch, err)
			return
		}

		for _, pr := range prs {
			log.Trace("Updating PR[%d]: composing new test task", pr.ID)
			if pr.Flow == issues_model.PullRequestFlowGithub {
				if err := PushToBaseRepo(ctx, pr); err != nil {
					log.Error("PushToBaseRepo: %v", err)
					continue
				}
			} else {
				continue
			}

			AddToTaskQueue(pr)
			comment, err := CreatePushPullComment(ctx, doer, pr, oldCommitID, newCommitID)
			if err == nil && comment != nil {
				notification.NotifyPullRequestPushCommits(ctx, doer, pr, comment)
			}
		}

		if isSync {
			requests := issues_model.PullRequestList(prs)
			if err = requests.LoadAttributes(); err != nil {
				log.Error("PullRequestList.LoadAttributes: %v", err)
			}
			if invalidationErr := checkForInvalidation(ctx, requests, repoID, doer, branch); invalidationErr != nil {
				log.Error("checkForInvalidation: %v", invalidationErr)
			}
			if err == nil {
				for _, pr := range prs {
					if newCommitID != "" && newCommitID != git.EmptySHA {
						changed, err := checkIfPRContentChanged(ctx, pr, oldCommitID, newCommitID)
						if err != nil {
							log.Error("checkIfPRContentChanged: %v", err)
						}
						if changed {
							// Mark old reviews as stale if diff to mergebase has changed
							if err := issues_model.MarkReviewsAsStale(pr.IssueID); err != nil {
								log.Error("MarkReviewsAsStale: %v", err)
							}

							// dismiss all approval reviews if protected branch rule item enabled.
							pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
							if err != nil {
								log.Error("GetFirstMatchProtectedBranchRule: %v", err)
							}
							if pb != nil && pb.DismissStaleApprovals {
								if err := DismissApprovalReviews(ctx, doer, pr); err != nil {
									log.Error("DismissApprovalReviews: %v", err)
								}
							}
						}
						if err := issues_model.MarkReviewsAsNotStale(pr.IssueID, newCommitID); err != nil {
							log.Error("MarkReviewsAsNotStale: %v", err)
						}
						divergence, err := GetDiverging(ctx, pr)
						if err != nil {
							log.Error("GetDiverging: %v", err)
						} else {
							err = pr.UpdateCommitDivergence(ctx, divergence.Ahead, divergence.Behind)
							if err != nil {
								log.Error("UpdateCommitDivergence: %v", err)
							}
						}
					}

					notification.NotifyPullRequestSynchronized(ctx, doer, pr)
				}
			}
		}

		log.Trace("AddTestPullRequestTask [base_repo_id: %d, base_branch: %s]: finding pull requests", repoID, branch)
		prs, err = issues_model.GetUnmergedPullRequestsByBaseInfo(repoID, branch)
		if err != nil {
			log.Error("Find pull requests [base_repo_id: %d, base_branch: %s]: %v", repoID, branch, err)
			return
		}
		for _, pr := range prs {
			divergence, err := GetDiverging(ctx, pr)
			if err != nil {
				if models.IsErrBranchDoesNotExist(err) && !git.IsBranchExist(ctx, pr.HeadRepo.RepoPath(), pr.HeadBranch) {
					log.Warn("Cannot test PR %s/%d: head_branch %s no longer exists", pr.BaseRepo.Name, pr.IssueID, pr.HeadBranch)
				} else {
					log.Error("GetDiverging: %v", err)
				}
			} else {
				err = pr.UpdateCommitDivergence(ctx, divergence.Ahead, divergence.Behind)
				if err != nil {
					log.Error("UpdateCommitDivergence: %v", err)
				}
			}
			AddToTaskQueue(pr)
		}
	})
}

// checkIfPRContentChanged checks if diff to target branch has changed by push
// A commit can be considered to leave the PR untouched if the patch/diff with its merge base is unchanged
func checkIfPRContentChanged(ctx context.Context, pr *issues_model.PullRequest, oldCommitID, newCommitID string) (hasChanged bool, err error) {
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		log.Error("CreateTemporaryRepoForPR %-v: %v", pr, err)
		return false, err
	}
	defer cancel()

	tmpRepo, err := git.OpenRepository(ctx, prCtx.tmpBasePath)
	if err != nil {
		return false, fmt.Errorf("OpenRepository: %w", err)
	}
	defer tmpRepo.Close()

	// Find the merge-base
	_, base, err := tmpRepo.GetMergeBase("", "base", "tracking")
	if err != nil {
		return false, fmt.Errorf("GetMergeBase: %w", err)
	}

	cmd := git.NewCommand(ctx, "diff", "--name-only", "-z").AddDynamicArguments(newCommitID, oldCommitID, base)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return false, fmt.Errorf("unable to open pipe for to run diff: %w", err)
	}

	if err := cmd.Run(&git.RunOpts{
		Dir:    prCtx.tmpBasePath,
		Stdout: stdoutWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer func() {
				_ = stdoutReader.Close()
			}()
			return util.IsEmptyReader(stdoutReader)
		},
	}); err != nil {
		if err == util.ErrNotEmpty {
			return true, nil
		}

		log.Error("Unable to run diff on %s %s %s in tempRepo for PR[%d]%s/%s...%s/%s: Error: %v",
			newCommitID, oldCommitID, base,
			pr.ID, pr.BaseRepo.FullName(), pr.BaseBranch, pr.HeadRepo.FullName(), pr.HeadBranch,
			err)

		return false, fmt.Errorf("Unable to run git diff --name-only -z %s %s %s: %w", newCommitID, oldCommitID, base, err)
	}

	return false, nil
}

// PushToBaseRepo pushes commits from branches of head repository to
// corresponding branches of base repository.
// FIXME: Only push branches that are actually updates?
func PushToBaseRepo(ctx context.Context, pr *issues_model.PullRequest) (err error) {
	return pushToBaseRepoHelper(ctx, pr, "")
}

func pushToBaseRepoHelper(ctx context.Context, pr *issues_model.PullRequest, prefixHeadBranch string) (err error) {
	log.Trace("PushToBaseRepo[%d]: pushing commits to base repo '%s'", pr.BaseRepoID, pr.GetGitRefName())

	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("Unable to load head repository for PR[%d] Error: %v", pr.ID, err)
		return err
	}
	headRepoPath := pr.HeadRepo.RepoPath()

	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to load base repository for PR[%d] Error: %v", pr.ID, err)
		return err
	}
	baseRepoPath := pr.BaseRepo.RepoPath()

	if err = pr.LoadIssue(ctx); err != nil {
		return fmt.Errorf("unable to load issue %d for pr %d: %w", pr.IssueID, pr.ID, err)
	}
	if err = pr.Issue.LoadPoster(ctx); err != nil {
		return fmt.Errorf("unable to load poster %d for pr %d: %w", pr.Issue.PosterID, pr.ID, err)
	}

	gitRefName := pr.GetGitRefName()

	if err := git.Push(ctx, headRepoPath, git.PushOptions{
		Remote: baseRepoPath,
		Branch: prefixHeadBranch + pr.HeadBranch + ":" + gitRefName,
		Force:  true,
		// Use InternalPushingEnvironment here because we know that pre-receive and post-receive do not run on a refs/pulls/...
		Env: repo_module.InternalPushingEnvironment(pr.Issue.Poster, pr.BaseRepo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) {
			// This should not happen as we're using force!
			log.Error("Unable to push PR head for %s#%d (%-v:%s) due to ErrPushOfDate: %v", pr.BaseRepo.FullName(), pr.Index, pr.BaseRepo, gitRefName, err)
			return err
		} else if git.IsErrPushRejected(err) {
			rejectErr := err.(*git.ErrPushRejected)
			log.Info("Unable to push PR head for %s#%d (%-v:%s) due to rejection:\nStdout: %s\nStderr: %s\nError: %v", pr.BaseRepo.FullName(), pr.Index, pr.BaseRepo, gitRefName, rejectErr.StdOut, rejectErr.StdErr, rejectErr.Err)
			return err
		} else if git.IsErrMoreThanOne(err) {
			if prefixHeadBranch != "" {
				log.Info("Can't push with %s%s", prefixHeadBranch, pr.HeadBranch)
				return err
			}
			log.Info("Retrying to push with %s%s", git.BranchPrefix, pr.HeadBranch)
			err = pushToBaseRepoHelper(ctx, pr, git.BranchPrefix)
			return err
		}
		log.Error("Unable to push PR head for %s#%d (%-v:%s) due to Error: %v", pr.BaseRepo.FullName(), pr.Index, pr.BaseRepo, gitRefName, err)
		return fmt.Errorf("Push: %s:%s %s:%s %w", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), gitRefName, err)
	}

	return nil
}

// UpdateRef update refs/pull/id/head directly for agit flow pull request
func UpdateRef(ctx context.Context, pr *issues_model.PullRequest) (err error) {
	log.Trace("UpdateRef[%d]: upgate pull request ref in base repo '%s'", pr.ID, pr.GetGitRefName())
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to load base repository for PR[%d] Error: %v", pr.ID, err)
		return err
	}

	_, _, err = git.NewCommand(ctx, "update-ref").AddDynamicArguments(pr.GetGitRefName(), pr.HeadCommitID).RunStdString(&git.RunOpts{Dir: pr.BaseRepo.RepoPath()})
	if err != nil {
		log.Error("Unable to update ref in base repository for PR[%d] Error: %v", pr.ID, err)
	}

	return err
}

type errlist []error

func (errs errlist) Error() string {
	if len(errs) > 0 {
		var buf strings.Builder
		for i, err := range errs {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
	return ""
}

// CloseBranchPulls close all the pull requests who's head branch is the branch
func CloseBranchPulls(doer *user_model.User, repoID int64, branch string) error {
	prs, err := issues_model.GetUnmergedPullRequestsByHeadInfo(repoID, branch)
	if err != nil {
		return err
	}

	prs2, err := issues_model.GetUnmergedPullRequestsByBaseInfo(repoID, branch)
	if err != nil {
		return err
	}

	prs = append(prs, prs2...)
	if err := issues_model.PullRequestList(prs).LoadAttributes(); err != nil {
		return err
	}

	var errs errlist
	for _, pr := range prs {
		if err = issue_service.ChangeStatus(pr.Issue, doer, "", true); err != nil && !issues_model.IsErrPullWasClosed(err) && !issues_model.IsErrDependenciesLeft(err) {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// CloseRepoBranchesPulls close all pull requests which head branches are in the given repository, but only whose base repo is not in the given repository
func CloseRepoBranchesPulls(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) error {
	branches, _, err := git.GetBranchesByPath(ctx, repo.RepoPath(), 0, 0)
	if err != nil {
		return err
	}

	var errs errlist
	for _, branch := range branches {
		prs, err := issues_model.GetUnmergedPullRequestsByHeadInfo(repo.ID, branch.Name)
		if err != nil {
			return err
		}

		if err = issues_model.PullRequestList(prs).LoadAttributes(); err != nil {
			return err
		}

		for _, pr := range prs {
			// If the base repository for this pr is this repository there is no need to close it
			// as it is going to be deleted anyway
			if pr.BaseRepoID == repo.ID {
				continue
			}
			if err = issue_service.ChangeStatus(pr.Issue, doer, "", true); err != nil && !issues_model.IsErrPullWasClosed(err) {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

var commitMessageTrailersPattern = regexp.MustCompile(`(?:^|\n\n)(?:[\w-]+[ \t]*:[^\n]+\n*(?:[ \t]+[^\n]+\n*)*)+$`)

// GetSquashMergeCommitMessages returns the commit messages between head and merge base (if there is one)
func GetSquashMergeCommitMessages(ctx context.Context, pr *issues_model.PullRequest) string {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("Cannot load issue %d for PR id %d: Error: %v", pr.IssueID, pr.ID, err)
		return ""
	}

	if err := pr.Issue.LoadPoster(ctx); err != nil {
		log.Error("Cannot load poster %d for pr id %d, index %d Error: %v", pr.Issue.PosterID, pr.ID, pr.Index, err)
		return ""
	}

	if pr.HeadRepo == nil {
		var err error
		pr.HeadRepo, err = repo_model.GetRepositoryByID(ctx, pr.HeadRepoID)
		if err != nil {
			log.Error("GetRepositoryByIdCtx[%d]: %v", pr.HeadRepoID, err)
			return ""
		}
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.HeadRepo.RepoPath())
	if err != nil {
		log.Error("Unable to open head repository: Error: %v", err)
		return ""
	}
	defer closer.Close()

	var headCommit *git.Commit
	if pr.Flow == issues_model.PullRequestFlowGithub {
		headCommit, err = gitRepo.GetBranchCommit(pr.HeadBranch)
	} else {
		pr.HeadCommitID, err = gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			log.Error("Unable to get head commit: %s Error: %v", pr.GetGitRefName(), err)
			return ""
		}
		headCommit, err = gitRepo.GetCommit(pr.HeadCommitID)
	}
	if err != nil {
		log.Error("Unable to get head commit: %s Error: %v", pr.HeadBranch, err)
		return ""
	}

	mergeBase, err := gitRepo.GetCommit(pr.MergeBase)
	if err != nil {
		log.Error("Unable to get merge base commit: %s Error: %v", pr.MergeBase, err)
		return ""
	}

	limit := setting.Repository.PullRequest.DefaultMergeMessageCommitsLimit

	commits, err := gitRepo.CommitsBetweenLimit(headCommit, mergeBase, limit, 0)
	if err != nil {
		log.Error("Unable to get commits between: %s %s Error: %v", pr.HeadBranch, pr.MergeBase, err)
		return ""
	}

	posterSig := pr.Issue.Poster.NewGitSig().String()

	uniqueAuthors := make(container.Set[string])
	authors := make([]string, 0, len(commits))
	stringBuilder := strings.Builder{}

	if !setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages {
		message := strings.TrimSpace(pr.Issue.Content)
		stringBuilder.WriteString(message)
		if stringBuilder.Len() > 0 {
			stringBuilder.WriteRune('\n')
			if !commitMessageTrailersPattern.MatchString(message) {
				stringBuilder.WriteRune('\n')
			}
		}
	}

	// commits list is in reverse chronological order
	first := true
	for i := len(commits) - 1; i >= 0; i-- {
		commit := commits[i]

		if setting.Repository.PullRequest.PopulateSquashCommentWithCommitMessages {
			maxSize := setting.Repository.PullRequest.DefaultMergeMessageSize
			if maxSize < 0 || stringBuilder.Len() < maxSize {
				var toWrite []byte
				if first {
					first = false
					toWrite = []byte(strings.TrimPrefix(commit.CommitMessage, pr.Issue.Title))
				} else {
					toWrite = []byte(commit.CommitMessage)
				}

				if len(toWrite) > maxSize-stringBuilder.Len() && maxSize > -1 {
					toWrite = append(toWrite[:maxSize-stringBuilder.Len()], "..."...)
				}
				if _, err := stringBuilder.Write(toWrite); err != nil {
					log.Error("Unable to write commit message Error: %v", err)
					return ""
				}

				if _, err := stringBuilder.WriteRune('\n'); err != nil {
					log.Error("Unable to write commit message Error: %v", err)
					return ""
				}
			}
		}

		authorString := commit.Author.String()
		if uniqueAuthors.Add(authorString) && authorString != posterSig {
			// Compare use account as well to avoid adding the same author multiple times
			// times when email addresses are private or multiple emails are used.
			commitUser, _ := user_model.GetUserByEmail(ctx, commit.Author.Email)
			if commitUser == nil || commitUser.ID != pr.Issue.Poster.ID {
				authors = append(authors, authorString)
			}
		}
	}

	// Consider collecting the remaining authors
	if limit >= 0 && setting.Repository.PullRequest.DefaultMergeMessageAllAuthors {
		skip := limit
		limit = 30
		for {
			commits, err := gitRepo.CommitsBetweenLimit(headCommit, mergeBase, limit, skip)
			if err != nil {
				log.Error("Unable to get commits between: %s %s Error: %v", pr.HeadBranch, pr.MergeBase, err)
				return ""

			}
			if len(commits) == 0 {
				break
			}
			for _, commit := range commits {
				authorString := commit.Author.String()
				if uniqueAuthors.Add(authorString) && authorString != posterSig {
					commitUser, _ := user_model.GetUserByEmail(ctx, commit.Author.Email)
					if commitUser == nil || commitUser.ID != pr.Issue.Poster.ID {
						authors = append(authors, authorString)
					}
				}
			}
			skip += limit
		}
	}

	for _, author := range authors {
		if _, err := stringBuilder.Write([]byte("Co-authored-by: ")); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
		if _, err := stringBuilder.Write([]byte(author)); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
		if _, err := stringBuilder.WriteRune('\n'); err != nil {
			log.Error("Unable to write to string builder Error: %v", err)
			return ""
		}
	}

	return stringBuilder.String()
}

// GetIssuesLastCommitStatus returns a map of issue ID to the most recent commit's latest status
func GetIssuesLastCommitStatus(ctx context.Context, issues issues_model.IssueList) (map[int64]*git_model.CommitStatus, error) {
	_, lastStatus, err := GetIssuesAllCommitStatus(ctx, issues)
	return lastStatus, err
}

// GetIssuesAllCommitStatus returns a map of issue ID to a list of all statuses for the most recent commit as well as a map of issue ID to only the commit's latest status
func GetIssuesAllCommitStatus(ctx context.Context, issues issues_model.IssueList) (map[int64][]*git_model.CommitStatus, map[int64]*git_model.CommitStatus, error) {
	if err := issues.LoadPullRequests(ctx); err != nil {
		return nil, nil, err
	}
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return nil, nil, err
	}

	var (
		gitRepos = make(map[int64]*git.Repository)
		res      = make(map[int64][]*git_model.CommitStatus)
		lastRes  = make(map[int64]*git_model.CommitStatus)
		err      error
	)
	defer func() {
		for _, gitRepo := range gitRepos {
			gitRepo.Close()
		}
	}()

	for _, issue := range issues {
		if !issue.IsPull {
			continue
		}
		gitRepo, ok := gitRepos[issue.RepoID]
		if !ok {
			gitRepo, err = git.OpenRepository(ctx, issue.Repo.RepoPath())
			if err != nil {
				log.Error("Cannot open git repository %-v for issue #%d[%d]. Error: %v", issue.Repo, issue.Index, issue.ID, err)
				continue
			}
			gitRepos[issue.RepoID] = gitRepo
		}

		statuses, lastStatus, err := getAllCommitStatus(gitRepo, issue.PullRequest)
		if err != nil {
			log.Error("getAllCommitStatus: cant get commit statuses of pull [%d]: %v", issue.PullRequest.ID, err)
			continue
		}
		res[issue.PullRequest.ID] = statuses
		lastRes[issue.PullRequest.ID] = lastStatus
	}
	return res, lastRes, nil
}

// getAllCommitStatus get pr's commit statuses.
func getAllCommitStatus(gitRepo *git.Repository, pr *issues_model.PullRequest) (statuses []*git_model.CommitStatus, lastStatus *git_model.CommitStatus, err error) {
	sha, shaErr := gitRepo.GetRefCommitID(pr.GetGitRefName())
	if shaErr != nil {
		return nil, nil, shaErr
	}

	statuses, _, err = git_model.GetLatestCommitStatus(db.DefaultContext, pr.BaseRepo.ID, sha, db.ListOptions{ListAll: true})
	lastStatus = git_model.CalcCommitStatus(statuses)
	return statuses, lastStatus, err
}

// IsHeadEqualWithBranch returns if the commits of branchName are available in pull request head
func IsHeadEqualWithBranch(ctx context.Context, pr *issues_model.PullRequest, branchName string) (bool, error) {
	var err error
	if err = pr.LoadBaseRepo(ctx); err != nil {
		return false, err
	}
	baseGitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		return false, err
	}
	defer closer.Close()

	baseCommit, err := baseGitRepo.GetBranchCommit(branchName)
	if err != nil {
		return false, err
	}

	if err = pr.LoadHeadRepo(ctx); err != nil {
		return false, err
	}
	var headGitRepo *git.Repository
	if pr.HeadRepoID == pr.BaseRepoID {
		headGitRepo = baseGitRepo
	} else {
		var closer io.Closer

		headGitRepo, closer, err = git.RepositoryFromContextOrOpen(ctx, pr.HeadRepo.RepoPath())
		if err != nil {
			return false, err
		}
		defer closer.Close()
	}

	var headCommit *git.Commit
	if pr.Flow == issues_model.PullRequestFlowGithub {
		headCommit, err = headGitRepo.GetBranchCommit(pr.HeadBranch)
		if err != nil {
			return false, err
		}
	} else {
		pr.HeadCommitID, err = baseGitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return false, err
		}
		if headCommit, err = baseGitRepo.GetCommit(pr.HeadCommitID); err != nil {
			return false, err
		}
	}
	return baseCommit.HasPreviousCommit(headCommit.ID)
}
