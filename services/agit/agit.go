// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agit

import (
	"context"
	"fmt"
	"os"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	notify_service "code.gitea.io/gitea/services/notify"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ProcReceive handle proc receive work
func ProcReceive(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, opts *private.HookOptions) ([]private.HookProcReceiveRefResult, error) {
	results := make([]private.HookProcReceiveRefResult, 0, len(opts.OldCommitIDs))
	forcePush := opts.GitPushOptions.Bool(private.GitPushOptionForcePush)
	topicBranch := opts.GitPushOptions["topic"]
	title := strings.TrimSpace(opts.GitPushOptions["title"])
	description := strings.TrimSpace(opts.GitPushOptions["description"])
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	userName := strings.ToLower(opts.UserName)

	pusher, err := user_model.GetUserByID(ctx, opts.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user. Error: %w", err)
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

		if !opts.RefFullNames[i].IsFor() {
			results = append(results, private.HookProcReceiveRefResult{
				IsNotMatched: true,
				OriginalRef:  opts.RefFullNames[i],
			})
			continue
		}

		baseBranchName := opts.RefFullNames[i].ForBranchName()
		currentTopicBranch := ""
		if !gitRepo.IsBranchExist(baseBranchName) {
			// try match refs/for/<target-branch>/<topic-branch>
			for p, v := range baseBranchName {
				if v == '/' && gitRepo.IsBranchExist(baseBranchName[:p]) && p != len(baseBranchName)-1 {
					currentTopicBranch = baseBranchName[p+1:]
					baseBranchName = baseBranchName[:p]
					break
				}
			}
		}

		if len(topicBranch) == 0 && len(currentTopicBranch) == 0 {
			results = append(results, private.HookProcReceiveRefResult{
				OriginalRef: opts.RefFullNames[i],
				OldOID:      opts.OldCommitIDs[i],
				NewOID:      opts.NewCommitIDs[i],
				Err:         "topic-branch is not set",
			})
			continue
		}

		if len(currentTopicBranch) == 0 {
			currentTopicBranch = topicBranch
		}

		// because different user maybe want to use same topic,
		// So it's better to make sure the topic branch name
		// has username prefix
		var headBranch string
		if !strings.HasPrefix(currentTopicBranch, userName+"/") {
			headBranch = userName + "/" + currentTopicBranch
		} else {
			headBranch = currentTopicBranch
		}

		pr, err := issues_model.GetUnmergedPullRequest(ctx, repo.ID, repo.ID, headBranch, baseBranchName, issues_model.PullRequestFlowAGit)
		if err != nil {
			if !issues_model.IsErrPullRequestNotExist(err) {
				return nil, fmt.Errorf("failed to get unmerged agit flow pull request in repository: %s Error: %w", repo.FullName(), err)
			}

			var commit *git.Commit
			if title == "" || description == "" {
				commit, err = gitRepo.GetCommit(opts.NewCommitIDs[i])
				if err != nil {
					return nil, fmt.Errorf("failed to get commit %s in repository: %s Error: %w", opts.NewCommitIDs[i], repo.FullName(), err)
				}
			}

			// create a new pull request
			if title == "" {
				title = strings.Split(commit.CommitMessage, "\n")[0]
			}
			if description == "" {
				_, description, _ = strings.Cut(commit.CommitMessage, "\n\n")
			}
			if description == "" {
				description = title
			}

			prIssue := &issues_model.Issue{
				RepoID:   repo.ID,
				Title:    title,
				PosterID: pusher.ID,
				Poster:   pusher,
				IsPull:   true,
				Content:  description,
			}

			pr := &issues_model.PullRequest{
				HeadRepoID:   repo.ID,
				BaseRepoID:   repo.ID,
				HeadBranch:   headBranch,
				HeadCommitID: opts.NewCommitIDs[i],
				BaseBranch:   baseBranchName,
				HeadRepo:     repo,
				BaseRepo:     repo,
				MergeBase:    "",
				Type:         issues_model.PullRequestGitea,
				Flow:         issues_model.PullRequestFlowAGit,
			}
			prOpts := &pull_service.NewPullRequestOptions{
				Repo:        repo,
				Issue:       prIssue,
				PullRequest: pr,
			}
			if err := pull_service.NewPullRequest(ctx, prOpts); err != nil {
				return nil, err
			}

			log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)

			results = append(results, private.HookProcReceiveRefResult{
				Ref:               pr.GetGitRefName(),
				OriginalRef:       opts.RefFullNames[i],
				OldOID:            objectFormat.EmptyObjectID().String(),
				NewOID:            opts.NewCommitIDs[i],
				IsCreatePR:        false, // AGit always creates a pull request so there is no point in prompting user to create one
				URL:               fmt.Sprintf("%s/pulls/%d", repo.HTMLURL(), pr.Index),
				ShouldShowMessage: setting.Git.PullRequestPushMessage && repo.AllowsPulls(ctx),
				HeadBranch:        headBranch,
			})
			continue
		}

		// update exist pull request
		if err := pr.LoadBaseRepo(ctx); err != nil {
			return nil, fmt.Errorf("unable to load base repository for PR[%d] Error: %w", pr.ID, err)
		}

		oldCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, fmt.Errorf("unable to get ref commit id in base repository for PR[%d] Error: %w", pr.ID, err)
		}

		if oldCommitID == opts.NewCommitIDs[i] {
			results = append(results, private.HookProcReceiveRefResult{
				OriginalRef: opts.RefFullNames[i],
				OldOID:      opts.OldCommitIDs[i],
				NewOID:      opts.NewCommitIDs[i],
				Err:         "new commit is same with old commit",
			})
			continue
		}

		if !forcePush.Value() {
			output, _, err := git.NewCommand("rev-list", "--max-count=1").
				AddDynamicArguments(oldCommitID, "^"+opts.NewCommitIDs[i]).
				RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath(), Env: os.Environ()})
			if err != nil {
				return nil, fmt.Errorf("failed to detect force push: %w", err)
			} else if len(output) > 0 {
				results = append(results, private.HookProcReceiveRefResult{
					OriginalRef: opts.RefFullNames[i],
					OldOID:      opts.OldCommitIDs[i],
					NewOID:      opts.NewCommitIDs[i],
					Err:         "request `force-push` push option",
				})
				continue
			}
		}

		pr.HeadCommitID = opts.NewCommitIDs[i]
		if err = pull_service.UpdateRef(ctx, pr); err != nil {
			return nil, fmt.Errorf("failed to update pull ref. Error: %w", err)
		}

		pull_service.AddToTaskQueue(ctx, pr)
		err = pr.LoadIssue(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load pull issue. Error: %w", err)
		}
		comment, err := pull_service.CreatePushPullComment(ctx, pusher, pr, oldCommitID, opts.NewCommitIDs[i])
		if err == nil && comment != nil {
			notify_service.PullRequestPushCommits(ctx, pusher, pr, comment)
		}
		notify_service.PullRequestSynchronized(ctx, pusher, pr)
		isForcePush := comment != nil && comment.IsForcePush

		results = append(results, private.HookProcReceiveRefResult{
			OldOID:            oldCommitID,
			NewOID:            opts.NewCommitIDs[i],
			Ref:               pr.GetGitRefName(),
			OriginalRef:       opts.RefFullNames[i],
			IsForcePush:       isForcePush,
			IsCreatePR:        false,
			URL:               fmt.Sprintf("%s/pulls/%d", repo.HTMLURL(), pr.Index),
			ShouldShowMessage: setting.Git.PullRequestPushMessage && repo.AllowsPulls(ctx),
		})
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
