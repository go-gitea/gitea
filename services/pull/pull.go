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
	issue_service "code.gitea.io/gitea/services/issue"
)

const issueMaxDupIndexAttempts = 3

// newPullRequest creates new pull request with labels for repository.
func newPullRequest(repo *models.Repository, pull *models.Issue, labelIDs []int64, uuids []string, pr *models.PullRequest, patch []byte) (err error) {
	// Retry several times in case INSERT fails due to duplicate key for (repo_id, index); see #7887
	i := 0
	for {
		if err = newPullRequestAttempt(repo, pull, labelIDs, uuids, pr, patch); err == nil {
			return nil
		}
		if !models.IsErrNewIssueInsert(err) {
			return err
		}
		if i++; i == issueMaxDupIndexAttempts {
			break
		}
		log.Error("NewPullRequest: error attempting to insert the new issue; will retry. Original error: %v", err)
	}
	return fmt.Errorf("NewPullRequest: too many errors attempting to insert the new issue. Last error was: %v", err)
}

func newPullRequestAttempt(repo *models.Repository, pull *models.Issue, labelIDs []int64, uuids []string, pr *models.PullRequest, patch []byte) (err error) {
	ctx, commiter, err := models.TxDBContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err = models.NewIssue(ctx, repo, pull, labelIDs, uuids); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) || models.IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %v", err)
	}

	pr.Index = pull.Index
	pr.BaseRepo = repo
	pr.Status = models.PullRequestStatusChecking
	if len(patch) > 0 {
		if err = savePatch(ctx, repo, pr.Index, patch); err != nil {
			return fmt.Errorf("SavePatch: %v", err)
		}

		if err = testPatch(pr, ctx); err != nil {
			return fmt.Errorf("testPatch: %v", err)
		}
	}
	// No conflict appears after test means mergeable.
	if pr.Status == models.PullRequestStatusChecking {
		pr.Status = models.PullRequestStatusMergeable
	}

	pr.IssueID = pull.ID
	if err = models.Insert(ctx, pr); err != nil {
		return fmt.Errorf("insert pull repo: %v", err)
	}

	if err = commiter.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// NewPullRequest creates new pull request with labels for repository.
func NewPullRequest(repo *models.Repository, pull *models.Issue, labelIDs []int64, uuids []string, pr *models.PullRequest, patch []byte, assigneeIDs []int64) error {
	if err := newPullRequest(repo, pull, labelIDs, uuids, pr, patch); err != nil {
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
		gitRepo.Close()
	}()
	return nil
}

func addHeadRepoTasks(prs []*models.PullRequest) {
	for _, pr := range prs {
		log.Trace("addHeadRepoTasks[%d]: composing new test task", pr.ID)
		if err := UpdatePatch(pr); err != nil {
			log.Error("UpdatePatch: %v", err)
			continue
		} else if err := pr.PushToBaseRepo(); err != nil {
			log.Error("PushToBaseRepo: %v", err)
			continue
		}

		AddToTaskQueue(pr)
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
				notification.NotifyPullRequestSynchronized(doer, pr)
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
		AddToTaskQueue(pr)
	}
}
