// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package agit

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/private"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ProcReceive handle proc receive work
func ProcReceive(ctx *context.PrivateContext, opts *private.HookOptions) []private.HookProcReceiveRefResult {
	// TODO: Add more options?
	var (
		topicBranch string
		title       string
		description string
		forcePush   bool
	)

	results := make([]private.HookProcReceiveRefResult, 0, len(opts.OldCommitIDs))
	repo := ctx.Repo.Repository
	gitRepo := ctx.Repo.GitRepo
	ownerName := ctx.Repo.Repository.OwnerName
	repoName := ctx.Repo.Repository.Name

	topicBranch = opts.GitPushOptions["topic"]
	_, forcePush = opts.GitPushOptions["force-push"]

	for i := range opts.OldCommitIDs {
		if opts.NewCommitIDs[i] == git.EmptySHA {
			results = append(results, private.HookProcReceiveRefResult{
				OriginalRef: opts.RefFullNames[i],
				OldOID:      opts.OldCommitIDs[i],
				NewOID:      opts.NewCommitIDs[i],
				Err:         "Can't delete not exist branch",
			})
			continue
		}

		if !strings.HasPrefix(opts.RefFullNames[i], git.PullRequestPrefix) {
			results = append(results, private.HookProcReceiveRefResult{
				IsNotMatched: true,
				OriginalRef:  opts.RefFullNames[i],
			})
			continue
		}

		baseBranchName := opts.RefFullNames[i][len(git.PullRequestPrefix):]
		curentTopicBranch := ""
		if !gitRepo.IsBranchExist(baseBranchName) {
			// try match refs/for/<target-branch>/<topic-branch>
			for p, v := range baseBranchName {
				if v == '/' && gitRepo.IsBranchExist(baseBranchName[:p]) && p != len(baseBranchName)-1 {
					curentTopicBranch = baseBranchName[p+1:]
					baseBranchName = baseBranchName[:p]
					break
				}
			}
		}

		if len(topicBranch) == 0 && len(curentTopicBranch) == 0 {
			results = append(results, private.HookProcReceiveRefResult{
				OriginalRef: opts.RefFullNames[i],
				OldOID:      opts.OldCommitIDs[i],
				NewOID:      opts.NewCommitIDs[i],
				Err:         "topic-branch is not set",
			})
			continue
		}

		headBranch := ""
		userName := strings.ToLower(opts.UserName)

		if len(curentTopicBranch) == 0 {
			curentTopicBranch = topicBranch
		}

		// because different user maybe want to use same topic,
		// So it's better to make sure the topic branch name
		// has user name prefix
		if !strings.HasPrefix(curentTopicBranch, userName+"/") {
			headBranch = userName + "/" + curentTopicBranch
		} else {
			headBranch = curentTopicBranch
		}

		pr, err := models.GetUnmergedPullRequest(repo.ID, repo.ID, headBranch, baseBranchName, models.PullRequestFlowAGit)
		if err != nil {
			if !models.IsErrPullRequestNotExist(err) {
				log.Error("Failed to get unmerged agit flow pull request in repository: %s/%s Error: %v", ownerName, repoName, err)
				ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
					"Err": fmt.Sprintf("Failed to get unmerged agit flow pull request in repository: %s/%s Error: %v", ownerName, repoName, err),
				})
				return nil
			}

			// create a new pull request
			if len(title) == 0 {
				has := false
				title, has = opts.GitPushOptions["title"]
				if !has || len(title) == 0 {
					commit, err := gitRepo.GetCommit(opts.NewCommitIDs[i])
					if err != nil {
						log.Error("Failed to get commit %s in repository: %s/%s Error: %v", opts.NewCommitIDs[i], ownerName, repoName, err)
						ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
							"Err": fmt.Sprintf("Failed to get commit %s in repository: %s/%s Error: %v", opts.NewCommitIDs[i], ownerName, repoName, err),
						})
						return nil
					}
					title = strings.Split(commit.CommitMessage, "\n")[0]
				}
				description = opts.GitPushOptions["description"]
			}

			pusher, err := user_model.GetUserByID(opts.UserID)
			if err != nil {
				log.Error("Failed to get user. Error: %v", err)
				ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
					"Err": fmt.Sprintf("Failed to get user. Error: %v", err),
				})
				return nil
			}

			prIssue := &models.Issue{
				RepoID:   repo.ID,
				Title:    title,
				PosterID: pusher.ID,
				Poster:   pusher,
				IsPull:   true,
				Content:  description,
			}

			pr := &models.PullRequest{
				HeadRepoID:   repo.ID,
				BaseRepoID:   repo.ID,
				HeadBranch:   headBranch,
				HeadCommitID: opts.NewCommitIDs[i],
				BaseBranch:   baseBranchName,
				HeadRepo:     repo,
				BaseRepo:     repo,
				MergeBase:    "",
				Type:         models.PullRequestGitea,
				Flow:         models.PullRequestFlowAGit,
			}

			if err := pull_service.NewPullRequest(ctx, repo, prIssue, []int64{}, []string{}, pr, []int64{}); err != nil {
				if models.IsErrUserDoesNotHaveAccessToRepo(err) {
					ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err.Error())
					return nil
				}
				ctx.Error(http.StatusInternalServerError, "NewPullRequest", err.Error())
				return nil
			}

			log.Trace("Pull request created: %d/%d", repo.ID, prIssue.ID)

			results = append(results, private.HookProcReceiveRefResult{
				Ref:         pr.GetGitRefName(),
				OriginalRef: opts.RefFullNames[i],
				OldOID:      git.EmptySHA,
				NewOID:      opts.NewCommitIDs[i],
			})
			continue
		}

		// update exist pull request
		if err := pr.LoadBaseRepoCtx(ctx); err != nil {
			log.Error("Unable to load base repository for PR[%d] Error: %v", pr.ID, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Unable to load base repository for PR[%d] Error: %v", pr.ID, err),
			})
			return nil
		}

		oldCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			log.Error("Unable to get ref commit id in base repository for PR[%d] Error: %v", pr.ID, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Unable to get ref commit id in base repository for PR[%d] Error: %v", pr.ID, err),
			})
			return nil
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

		if !forcePush {
			output, _, err := git.NewCommand(ctx, "rev-list", "--max-count=1", oldCommitID, "^"+opts.NewCommitIDs[i]).RunStdString(&git.RunOpts{Dir: repo.RepoPath(), Env: os.Environ()})
			if err != nil {
				log.Error("Unable to detect force push between: %s and %s in %-v Error: %v", oldCommitID, opts.NewCommitIDs[i], repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Fail to detect force push: %v", err),
				})
				return nil
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
			log.Error("Failed to update pull ref. Error: %v", err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Failed to update pull ref. Error: %v", err),
			})
			return nil
		}

		pull_service.AddToTaskQueue(pr)
		pusher, err := user_model.GetUserByID(opts.UserID)
		if err != nil {
			log.Error("Failed to get user. Error: %v", err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Failed to get user. Error: %v", err),
			})
			return nil
		}
		err = pr.LoadIssue()
		if err != nil {
			log.Error("Failed to load pull issue. Error: %v", err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Failed to load pull issue. Error: %v", err),
			})
			return nil
		}
		comment, err := models.CreatePushPullComment(ctx, pusher, pr, oldCommitID, opts.NewCommitIDs[i])
		if err == nil && comment != nil {
			notification.NotifyPullRequestPushCommits(pusher, pr, comment)
		}
		notification.NotifyPullRequestSynchronized(pusher, pr)
		isForcePush := comment != nil && comment.IsForcePush

		results = append(results, private.HookProcReceiveRefResult{
			OldOID:      oldCommitID,
			NewOID:      opts.NewCommitIDs[i],
			Ref:         pr.GetGitRefName(),
			OriginalRef: opts.RefFullNames[i],
			IsForcePush: isForcePush,
		})
	}

	return results
}

// UserNameChanged handle user name change for agit flow pull
func UserNameChanged(user *user_model.User, newName string) error {
	pulls, err := models.GetAllUnmergedAgitPullRequestByPoster(user.ID)
	if err != nil {
		return err
	}

	newName = strings.ToLower(newName)

	for _, pull := range pulls {
		pull.HeadBranch = strings.TrimPrefix(pull.HeadBranch, user.LowerName+"/")
		pull.HeadBranch = newName + "/" + pull.HeadBranch
		if err = pull.UpdateCols("head_branch"); err != nil {
			return err
		}
	}

	return nil
}
