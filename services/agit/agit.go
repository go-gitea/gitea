// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
)

func parseAgitPushOptionValue(s string) string {
	if base64Value, ok := strings.CutPrefix(s, "{base64}"); ok {
		decoded, err := base64.StdEncoding.DecodeString(base64Value)
		return util.Iif(err == nil, string(decoded), s)
	}
	return s
}

func GetAgitBranchInfo(ctx context.Context, repoID int64, baseBranchName string) (string, string, error) {
	baseBranchExist, err := git_model.IsBranchExist(ctx, repoID, baseBranchName)
	if err != nil {
		return "", "", err
	}
	if baseBranchExist {
		return baseBranchName, "", nil
	}

	// try match <target-branch>/<topic-branch>
	// refs/for have been trimmed to get baseBranchName
	for p, v := range baseBranchName {
		if v != '/' {
			continue
		}

		baseBranchExist, err := git_model.IsBranchExist(ctx, repoID, baseBranchName[:p])
		if err != nil {
			return "", "", err
		}
		if baseBranchExist {
			return baseBranchName[:p], baseBranchName[p+1:], nil
		}
	}

	return "", "", util.NewNotExistErrorf("base branch does not exist")
}

type handleProcReceiceContext struct {
	ctx          context.Context
	repo         *repo_model.Repository
	gitRepo      *git.Repository
	opts         *private.HookOptions
	objectFormat git.ObjectFormat

	userName string
	pusher   *user_model.User

	title       string
	description string
	topicBranch string

	forcePush optional.Option[bool]
}

func (context handleProcReceiceContext) handleUpdatePullRequest(pr *issues_model.PullRequest, newCommitID string, refFullName git.RefName) (*private.HookProcReceiveRefResult, error) {
	// update exist pull request
	if err := pr.LoadBaseRepo(context.ctx); err != nil {
		return nil, fmt.Errorf("unable to load base repository for PR[%d] Error: %w", pr.ID, err)
	}

	oldCommitID, err := context.gitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	if err != nil {
		return nil, fmt.Errorf("unable to get ref commit id in base repository for PR[%d] Error: %w", pr.ID, err)
	}

	if oldCommitID == newCommitID {
		return &private.HookProcReceiveRefResult{
			OriginalRef: refFullName,
			OldOID:      oldCommitID,
			NewOID:      newCommitID,
			Err:         "new commit is same with old commit",
		}, nil
	}

	if !context.forcePush.Value() {
		output, _, err := gitrepo.RunCmdString(context.ctx, context.repo,
			gitcmd.NewCommand("rev-list", "--max-count=1").
				AddDynamicArguments(oldCommitID, "^"+newCommitID),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to detect force push: %w", err)
		} else if len(output) > 0 {
			return &private.HookProcReceiveRefResult{
				OriginalRef: refFullName,
				OldOID:      oldCommitID,
				NewOID:      newCommitID,
				Err:         "request `force-push` push option",
			}, nil
		}
	}

	// Store old commit ID for review staleness checking
	oldHeadCommitID := pr.HeadCommitID

	pr.HeadCommitID = newCommitID
	if err = pull_service.UpdateRef(context.ctx, pr); err != nil {
		return nil, fmt.Errorf("failed to update pull ref. Error: %w", err)
	}

	// Mark existing reviews as stale when PR content changes (same as regular GitHub flow)
	if oldHeadCommitID != newCommitID {
		if err := issues_model.MarkReviewsAsStale(context.ctx, pr.IssueID); err != nil {
			log.Error("MarkReviewsAsStale: %v", err)
		}

		// Dismiss all approval reviews if protected branch rule item enabled
		pb, err := git_model.GetFirstMatchProtectedBranchRule(context.ctx, pr.BaseRepoID, pr.BaseBranch)
		if err != nil {
			log.Error("GetFirstMatchProtectedBranchRule: %v", err)
		}
		if pb != nil && pb.DismissStaleApprovals {
			if err := pull_service.DismissApprovalReviews(context.ctx, context.pusher, pr); err != nil {
				log.Error("DismissApprovalReviews: %v", err)
			}
		}

		// Mark reviews for the new commit as not stale
		if err := issues_model.MarkReviewsAsNotStale(context.ctx, pr.IssueID, newCommitID); err != nil {
			log.Error("MarkReviewsAsNotStale: %v", err)
		}
	}

	pull_service.StartPullRequestCheckImmediately(context.ctx, pr)
	err = pr.LoadIssue(context.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load pull issue. Error: %w", err)
	}
	comment, err := pull_service.CreatePushPullComment(context.ctx, context.pusher, pr, oldCommitID, newCommitID, context.forcePush.Value())
	if err == nil && comment != nil {
		notify_service.PullRequestPushCommits(context.ctx, context.pusher, pr, comment)
	}
	notify_service.PullRequestSynchronized(context.ctx, context.pusher, pr)
	isForcePush := comment != nil && comment.IsForcePush

	return &private.HookProcReceiveRefResult{
		OldOID:            oldCommitID,
		NewOID:            newCommitID,
		Ref:               pr.GetGitHeadRefName(),
		OriginalRef:       refFullName,
		IsForcePush:       isForcePush,
		IsCreatePR:        false,
		URL:               fmt.Sprintf("%s/pulls/%d", context.repo.HTMLURL(), pr.Index),
		ShouldShowMessage: setting.Git.PullRequestPushMessage && context.repo.AllowsPulls(context.ctx),
	}, nil
}

func (context *handleProcReceiceContext) handleCreatePullRequest(newCommitID, headBranch, baseBranchName string, refFullName git.RefName) (*private.HookProcReceiveRefResult, error) {
	var (
		commit *git.Commit
		err    error
	)

	if context.title == "" || context.description == "" {
		commit, err = context.gitRepo.GetCommit(newCommitID)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit %s in repository: %s Error: %w", newCommitID, context.repo.FullName(), err)
		}
	}

	// create a new pull request
	if context.title == "" {
		context.title = strings.Split(commit.CommitMessage, "\n")[0]
	}
	if context.description == "" {
		_, context.description, _ = strings.Cut(commit.CommitMessage, "\n\n")
	}
	if context.description == "" {
		context.description = context.title
	}

	prIssue := &issues_model.Issue{
		RepoID:   context.repo.ID,
		Title:    context.title,
		PosterID: context.pusher.ID,
		Poster:   context.pusher,
		IsPull:   true,
		Content:  context.description,
	}

	pr := &issues_model.PullRequest{
		HeadRepoID:   context.repo.ID,
		BaseRepoID:   context.repo.ID,
		HeadBranch:   headBranch,
		HeadCommitID: newCommitID,
		BaseBranch:   baseBranchName,
		HeadRepo:     context.repo,
		BaseRepo:     context.repo,
		MergeBase:    "",
		Type:         issues_model.PullRequestGitea,
		Flow:         issues_model.PullRequestFlowAGit,
	}
	prOpts := &pull_service.NewPullRequestOptions{
		Repo:        context.repo,
		Issue:       prIssue,
		PullRequest: pr,
	}
	if err := pull_service.NewPullRequest(context.ctx, prOpts); err != nil {
		return nil, err
	}

	log.Trace("Pull request created: %d/%d", context.repo.ID, prIssue.ID)

	return &private.HookProcReceiveRefResult{
		Ref:               pr.GetGitHeadRefName(),
		OriginalRef:       refFullName,
		OldOID:            context.objectFormat.EmptyObjectID().String(),
		NewOID:            newCommitID,
		IsCreatePR:        false, // AGit always creates a pull request so there is no point in prompting user to create one
		URL:               fmt.Sprintf("%s/pulls/%d", context.repo.HTMLURL(), pr.Index),
		ShouldShowMessage: setting.Git.PullRequestPushMessage && context.repo.AllowsPulls(context.ctx),
		HeadBranch:        headBranch,
	}, nil
}

func (context *handleProcReceiceContext) handleFor(refFullName git.RefName, oldCommitID, newCommitID string) (*private.HookProcReceiveRefResult, error) {
	baseBranchName, currentTopicBranch, err := GetAgitBranchInfo(context.ctx, context.repo.ID, refFullName.ForBranchName())
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) {
			return nil, fmt.Errorf("failed to get branch information. Error: %w", err)
		}

		// If branch does not exist, we can continue
		return &private.HookProcReceiveRefResult{
			OriginalRef: refFullName,
			OldOID:      oldCommitID,
			NewOID:      newCommitID,
			Err:         "base-branch does not exist",
		}, nil
	}

	if len(context.topicBranch) == 0 && len(currentTopicBranch) == 0 {
		return &private.HookProcReceiveRefResult{
			OriginalRef: refFullName,
			OldOID:      oldCommitID,
			NewOID:      newCommitID,
			Err:         "topic-branch is not set",
		}, nil
	}

	if len(currentTopicBranch) == 0 {
		currentTopicBranch = context.topicBranch
	}

	// because different user maybe want to use same topic,
	// So it's better to make sure the topic branch name
	// has username prefix
	var headBranch string
	if !strings.HasPrefix(currentTopicBranch, context.userName+"/") {
		headBranch = context.userName + "/" + currentTopicBranch
	} else {
		headBranch = currentTopicBranch
	}

	pr, err := issues_model.GetUnmergedPullRequest(context.ctx, context.repo.ID, context.repo.ID, headBranch, baseBranchName, issues_model.PullRequestFlowAGit)
	if err != nil {
		if !issues_model.IsErrPullRequestNotExist(err) {
			return nil, fmt.Errorf("failed to get unmerged agit flow pull request in repository: %s Error: %w", context.repo.FullName(), err)
		}

		return context.handleCreatePullRequest(newCommitID, headBranch, baseBranchName, refFullName)
	}

	return context.handleUpdatePullRequest(pr, newCommitID, refFullName)
}

func (context *handleProcReceiceContext) handleForReview(refFullName git.RefName, oldCommitID, newCommitID string) (*private.HookProcReceiveRefResult, error) {
	// try match refs/for-review/<pull index>
	pullIndex, err := strconv.ParseInt(strings.TrimPrefix(string(refFullName), git.ForReviewPrefix), 10, 64)
	if err != nil {
		return &private.HookProcReceiveRefResult{
			OriginalRef: refFullName,
			OldOID:      oldCommitID,
			NewOID:      newCommitID,
			Err:         "Unknow pull request index",
		}, nil
	}

	log.Trace("Pull request index: %d", pullIndex)
	pull, err := issues_model.GetPullRequestByIndex(context.ctx, context.repo.ID, pullIndex)
	if err != nil {
		return &private.HookProcReceiveRefResult{
			OriginalRef: refFullName,
			OldOID:      oldCommitID,
			NewOID:      newCommitID,
			Err:         "Unknow pull request index",
		}, nil
	}

	return context.handleUpdatePullRequest(pull, newCommitID, refFullName)
}

// ProcReceive handle proc receive work
func ProcReceive(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, opts *private.HookOptions) ([]private.HookProcReceiveRefResult, error) {
	results := make([]private.HookProcReceiveRefResult, 0, len(opts.OldCommitIDs))
	forcePush := opts.GitPushOptions.Bool(private.GitPushOptionForcePush)
	topicBranch := opts.GitPushOptions["topic"]

	// some options are base64-encoded with "{base64}" prefix if they contain new lines
	// other agit push options like "issue", "reviewer" and "cc" are not supported
	title := parseAgitPushOptionValue(opts.GitPushOptions["title"])
	description := parseAgitPushOptionValue(opts.GitPushOptions["description"])

	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	userName := strings.ToLower(opts.UserName)

	pusher, err := user_model.GetUserByID(ctx, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user. Error: %w", err)
	}

	hctx := handleProcReceiceContext{
		ctx:          ctx,
		repo:         repo,
		gitRepo:      gitRepo,
		opts:         opts,
		objectFormat: objectFormat,

		userName: userName,
		pusher:   pusher,

		title:       title,
		description: description,
		topicBranch: topicBranch,

		forcePush: forcePush,
	}

	for i := range opts.OldCommitIDs {
		if opts.NewCommitIDs[i] == objectFormat.EmptyObjectID().String() {
			results = append(results, private.HookProcReceiveRefResult{
				OriginalRef: opts.RefFullNames[i],
				OldOID:      opts.OldCommitIDs[i],
				NewOID:      opts.NewCommitIDs[i],
				Err:         "Can't delete not exist branch",
			})
			continue
		}

		var (
			res *private.HookProcReceiveRefResult
			err error
		)

		if opts.RefFullNames[i].IsForReview() {
			res, err = hctx.handleForReview(opts.RefFullNames[i], opts.OldCommitIDs[i], opts.NewCommitIDs[i])
		} else if opts.RefFullNames[i].IsFor() {
			res, err = hctx.handleFor(opts.RefFullNames[i], opts.OldCommitIDs[i], opts.NewCommitIDs[i])
		} else {
			results = append(results, private.HookProcReceiveRefResult{
				IsNotMatched: true,
				OriginalRef:  opts.RefFullNames[i],
			})
			continue
		}

		if err != nil {
			return nil, err
		}

		results = append(results, *res)
	}

	return results, nil
}

// UserNameChanged handle user name change for agit flow pull
func UserNameChanged(ctx context.Context, user *user_model.User, newName string) error {
	pulls, err := issues_model.GetAllUnmergedAgitPullRequestByPoster(ctx, user.ID)
	if err != nil {
		return err
	}

	newName = strings.ToLower(newName)

	for _, pull := range pulls {
		pull.HeadBranch = strings.TrimPrefix(pull.HeadBranch, user.LowerName+"/")
		pull.HeadBranch = newName + "/" + pull.HeadBranch
		if err = pull.UpdateCols(ctx, "head_branch"); err != nil {
			return err
		}
	}

	return nil
}
