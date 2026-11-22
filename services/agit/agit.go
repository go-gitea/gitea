// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
)

const SshInfoJson = `{"type":"agit","version":1}`

func GetPullForReview(ctx context.Context, repoID int64, ref git.RefName, doer *user_model.User, perm access.Permission) (*issues_model.PullRequest, error) {
	index, ok := ref.ForReviewIndex()
	if !ok {
		return nil, util.NewPermissionDeniedErrorf("Invalid pull request index")
	}
	pr, err := issues_model.GetPullRequestByIndex(ctx, repoID, index)
	if issues_model.IsErrPullRequestNotExist(err) {
		return nil, util.NewPermissionDeniedErrorf("Pull request does not exist")
	}
	if err != nil {
		return nil, err
	}
	if !pr.IsAgitFlow() || pr.Issue.IsClosed || pr.HasMerged {
		return nil, util.NewPermissionDeniedErrorf("Pull request is not an open AGit pull request")
	}
	if doer.ID != pr.Issue.PosterID {
		if !perm.CanWrite(unit.TypeCode) {
			return nil, util.NewPermissionDeniedErrorf("User permission denied for updating pull request")
		}
		if !pr.AllowMaintainerEdit {
			return nil, util.NewPermissionDeniedErrorf("The pull request author has not allowed edits by maintainers")
		}
	}
	return pr, nil
}

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

type ProcReceiveOptions struct {
	OldCommitIDs []string
	NewCommitIDs []string
	RefFullNames []git.RefName

	GitPushOptions private.GitPushOptions

	Doer *user_model.User
}

func getOrCreatePullForBranch(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, perm access.Permission, opts *ProcReceiveOptions, i int) (*issues_model.PullRequest, bool, error) {
	baseBranch, topicBranch, err := GetAgitBranchInfo(ctx, repo.ID, opts.RefFullNames[i].ForBranchName())
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, false, util.NewNotExistErrorf("base-branch does not exist")
		}
		return nil, false, err
	}

	if topicBranch == "" {
		topicBranch = opts.GitPushOptions["topic"]
	}
	if topicBranch == "" {
		return nil, false, util.NewInvalidArgumentErrorf("topic-branch is not set")
	}

	pusher := opts.Doer
	headBranch := topicBranch
	userPrefix := strings.ToLower(pusher.Name) + "/"
	if !strings.HasPrefix(headBranch, userPrefix) {
		headBranch = userPrefix + headBranch
	}

	pr, err := issues_model.GetUnmergedPullRequest(ctx, repo.ID, repo.ID, headBranch, baseBranch, issues_model.PullRequestFlowAGit)
	if !issues_model.IsErrPullRequestNotExist(err) {
		return pr, false, err
	}

	// Push options containing newlines can use the "{base64}" prefix.
	title := parseAgitPushOptionValue(opts.GitPushOptions["title"])
	description := parseAgitPushOptionValue(opts.GitPushOptions["description"])
	if title == "" || description == "" {
		commit, err := gitRepo.GetCommit(ctx, opts.NewCommitIDs[i])
		if err != nil {
			return nil, false, err
		}
		title = util.IfZero(title, commit.MessageTitle())
		description = util.IfZero(description, commit.MessageBody())
	}

	issue := &issues_model.Issue{
		RepoID:   repo.ID,
		Title:    title,
		PosterID: pusher.ID,
		Poster:   pusher,
		IsPull:   true,
		Content:  description,
	}
	pr = &issues_model.PullRequest{
		HeadRepoID:   repo.ID,
		BaseRepoID:   repo.ID,
		HeadBranch:   headBranch,
		HeadCommitID: opts.NewCommitIDs[i],
		BaseBranch:   baseBranch,
		HeadRepo:     repo,
		BaseRepo:     repo,
		Type:         issues_model.PullRequestGitea,
		Flow:         issues_model.PullRequestFlowAGit,
	}
	if err := pull_service.NewPullRequest(ctx, perm, &pull_service.NewPullRequestOptions{
		Repo:        repo,
		Issue:       issue,
		PullRequest: pr,
	}); err != nil {
		return nil, false, err
	}

	log.Trace("Pull request created: %d/%d", repo.ID, issue.ID)
	return pr, true, nil
}

// ProcReceive handle proc receive work
func ProcReceive(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, perm access.Permission, opts *ProcReceiveOptions) ([]private.HookProcReceiveRefResult, error) {
	if len(opts.RefFullNames) > 1 && slices.ContainsFunc(opts.RefFullNames, git.RefName.IsForReview) {
		return nil, util.NewInvalidArgumentErrorf(private.AgitReviewSingleRefMessage)
	}

	results := make([]private.HookProcReceiveRefResult, 0, len(opts.OldCommitIDs))
	forcePush := opts.GitPushOptions.Bool(private.GitPushOptionForcePush).Value()
	emptyOID := git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String()

	for i, ref := range opts.RefFullNames {
		result := private.HookProcReceiveRefResult{
			OriginalRef: ref,
			OldOID:      opts.OldCommitIDs[i],
			NewOID:      opts.NewCommitIDs[i],
		}
		if result.NewOID == emptyOID {
			result.Err = "Can't delete not exist branch"
			results = append(results, result)
			continue
		}
		if !ref.IsFor() && !ref.IsForReview() {
			result.IsNotMatched = true
			results = append(results, result)
			continue
		}

		var pr *issues_model.PullRequest
		var created bool
		var err error
		if ref.IsForReview() {
			pr, err = GetPullForReview(ctx, repo.ID, ref, opts.Doer, perm)
		} else {
			pr, created, err = getOrCreatePullForBranch(ctx, repo, gitRepo, perm, opts, i)
		}
		if err == nil && !created {
			result.OldOID, err = updatePullForPush(ctx, repo, gitRepo, opts.Doer, pr, result.NewOID, forcePush)
			result.IsForcePush = forcePush
		}
		if err != nil {
			msg, _ := util.ErrorUnwrapForUser(err)
			if msg == "" {
				return nil, fmt.Errorf("ProcReceive: %w", err)
			}
			result.Err = msg
		} else {
			result.Ref = pr.GetGitHeadRefName()
			result.URL = fmt.Sprintf("%s/pulls/%d", repo.HTMLURL(), pr.Index)
			result.ShouldShowMessage = setting.Git.PullRequestPushMessage && repo.AllowsPulls(ctx)
			if created {
				result.OldOID = emptyOID
				result.HeadBranch = pr.HeadBranch
			}
		}
		results = append(results, result)
	}

	return results, nil
}

func updatePullForPush(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, pusher *user_model.User, pr *issues_model.PullRequest, newCommitID string, forcePush bool) (string, error) {
	if err := pr.LoadIssue(ctx); err != nil {
		return "", err
	}

	oldCommitID, err := gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		return "", fmt.Errorf("unable to get ref commit id for PR[%d]: %w", pr.ID, err)
	}
	if oldCommitID == newCommitID {
		return "", util.NewInvalidArgumentErrorf("new commit is same with old commit")
	}
	if !forcePush {
		output, _, err := gitcmd.NewCommand("rev-list", "--max-count=1").
			AddDynamicArguments(oldCommitID, "^"+newCommitID).WithRepo(repo).RunStdString(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to detect force push: %w", err)
		}
		if len(output) > 0 {
			return "", util.NewInvalidArgumentErrorf("request `force-push` push option")
		}
	}

	pr.HeadCommitID = newCommitID
	if err := pull_service.UpdateRefForAgit(ctx, pr, oldCommitID); err != nil {
		return "", err
	}
	if err := pull_service.DisableAutoMergeIfNotWriter(ctx, pusher, pr, "pushed_by_non_writer"); err != nil {
		return "", err
	}
	if err := pull_service.SyncCommitDivergence(ctx, pr); err != nil {
		log.Error("SyncCommitDivergence: %v", err)
	}

	if err := issues_model.MarkReviewsAsStale(ctx, pr.IssueID); err != nil {
		log.Error("MarkReviewsAsStale: %v", err)
	}
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		log.Error("GetFirstMatchProtectedBranchRule: %v", err)
	}
	if pb != nil && pb.DismissStaleApprovals {
		if err := pull_service.DismissApprovalReviews(ctx, pusher, pr); err != nil {
			log.Error("DismissApprovalReviews: %v", err)
		}
	}
	if err := issues_model.MarkReviewsAsNotStale(ctx, pr.IssueID, newCommitID); err != nil {
		log.Error("MarkReviewsAsNotStale: %v", err)
	}

	pull_service.StartPullRequestCheckImmediately(ctx, pr)
	comment, created, err := pull_service.CreatePushPullComment(ctx, pusher, pr, oldCommitID, newCommitID, forcePush)
	if err != nil {
		log.Error("CreatePushPullComment: %v", err)
	} else if created {
		notify_service.PullRequestPushCommits(ctx, pusher, pr, comment)
	}
	notify_service.PullRequestSynchronized(ctx, pusher, pr, oldCommitID, newCommitID)
	return oldCommitID, nil
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
