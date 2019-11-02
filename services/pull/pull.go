// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/webhook"
	issue_service "code.gitea.io/gitea/services/issue"
)

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(repo *models.Repository, pull *models.Issue, labelIDs []int64, uuids []string, pr *models.PullRequest, patch []byte, assigneeIDs []int64) error {
	if err := models.NewPullRequest(repo, pull, labelIDs, uuids, pr, patch); err != nil {
		return err
	}

	for _, assigneeID := range assigneeIDs {
		if err := issue_service.AddAssigneeIfNotAssigned(pull, pull.Poster, assigneeID); err != nil {
			return err
		}
	}

	pr.Issue = pull
	pull.PullRequest = pr

	notification.NotifyNewPullRequest(pr)

	return nil
}

func checkForInvalidation(requests models.PullRequestList, repoID int64, doer *models.User, branch string) error {
	repo, err := models.GetRepositoryByID(repoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %v", err)
	}
	go func() {
		err := requests.InvalidateCodeComments(doer, gitRepo, branch)
		if err != nil {
			log.Error("PullRequestList.InvalidateCodeComments: %v", err)
		}
	}()
	return nil
}

func addHeadRepoTasks(prs []*models.PullRequest) {
	for _, pr := range prs {
		log.Trace("addHeadRepoTasks[%d]: composing new test task", pr.ID)
		if err := pr.UpdatePatch(); err != nil {
			log.Error("UpdatePatch: %v", err)
			continue
		} else if err := pr.PushToBaseRepo(); err != nil {
			log.Error("PushToBaseRepo: %v", err)
			continue
		}

		pr.AddToTaskQueue()
	}
}

// AddTestPullRequestTask adds new test tasks by given head/base repository and head/base branch,
// and generate new patch for testing as needed.
func AddTestPullRequestTask(doer *models.User, repoID int64, branch string, isSync bool) {
	log.Trace("AddTestPullRequestTask [head_repo_id: %d, head_branch: %s]: finding pull requests", repoID, branch)
	prs, err := models.GetUnmergedPullRequestsByHeadInfo(repoID, branch)
	if err != nil {
		log.Error("Find pull requests [head_repo_id: %d, head_branch: %s]: %v", repoID, branch, err)
		return
	}

	if isSync {
		requests := models.PullRequestList(prs)
		if err = requests.LoadAttributes(); err != nil {
			log.Error("PullRequestList.LoadAttributes: %v", err)
		}
		if invalidationErr := checkForInvalidation(requests, repoID, doer, branch); invalidationErr != nil {
			log.Error("checkForInvalidation: %v", invalidationErr)
		}
		if err == nil {
			for _, pr := range prs {
				pr.Issue.PullRequest = pr
				if err = pr.Issue.LoadAttributes(); err != nil {
					log.Error("LoadAttributes: %v", err)
					continue
				}
				if err = webhook.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
					Action:      api.HookIssueSynchronized,
					Index:       pr.Issue.Index,
					PullRequest: pr.Issue.PullRequest.APIFormat(),
					Repository:  pr.Issue.Repo.APIFormat(models.AccessModeNone),
					Sender:      doer.APIFormat(),
				}); err != nil {
					log.Error("PrepareWebhooks [pull_id: %v]: %v", pr.ID, err)
					continue
				}
			}
		}

	}

	addHeadRepoTasks(prs)

	log.Trace("AddTestPullRequestTask [base_repo_id: %d, base_branch: %s]: finding pull requests", repoID, branch)
	prs, err = models.GetUnmergedPullRequestsByBaseInfo(repoID, branch)
	if err != nil {
		log.Error("Find pull requests [base_repo_id: %d, base_branch: %s]: %v", repoID, branch, err)
		return
	}
	for _, pr := range prs {
		pr.AddToTaskQueue()
	}
}
