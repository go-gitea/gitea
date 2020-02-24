// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// Update updates pull request with base branch.
func Update(pull *models.PullRequest, doer *models.User, message string) error {
	//use merge functions but switch repo's and branch's
	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	diffCount, err := GetDiverging(pull)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pull.Index)
	}

	_, err = rawMerge(pr, doer, models.MergeStyleMerge, message)

	defer func() {
		go AddTestPullRequestTask(doer, pr.HeadRepo.ID, pr.HeadBranch, false, "", "")
	}()

	return err
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
func IsUserAllowedToUpdate(pull *models.PullRequest, user *models.User) (bool, error) {
	headRepoPerm, err := models.GetUserRepoPermission(pull.HeadRepo, user)
	if err != nil {
		return false, err
	}

	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}
	return IsUserAllowedToMerge(pr, headRepoPerm, user)
}

// GetDiverging determines how many commits a PR is ahead or behind the PR base branch
func GetDiverging(pr *models.PullRequest) (*git.DivergeObject, error) {
	log.Trace("PushToBaseRepo[%d]: pushing commits to base repo '%s'", pr.BaseRepoID, pr.GetGitRefName())
	if err := pr.LoadBaseRepo(); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepo(); err != nil {
		return nil, err
	}

	headRepoPath := pr.HeadRepo.RepoPath()
	headGitRepo, err := git.OpenRepository(headRepoPath)
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer headGitRepo.Close()

	if pr.IsSameRepo() {
		diff, err := git.GetDivergingCommits(pr.HeadRepo.RepoPath(), pr.BaseBranch, pr.HeadBranch)
		return &diff, err
	}

	tmpRemoteName := fmt.Sprintf("tmp-pull-%d-base", pr.ID)
	if err = headGitRepo.AddRemote(tmpRemoteName, pr.BaseRepo.RepoPath(), true); err != nil {
		return nil, fmt.Errorf("headGitRepo.AddRemote: %v", err)
	}
	// Make sure to remove the remote even if the push fails
	defer func() {
		if err := headGitRepo.RemoveRemote(tmpRemoteName); err != nil {
			log.Error("CountDiverging: RemoveRemote: %s", err)
		}
	}()

	// $(git rev-list --count tmp-pull-1-base/master..feature) commits ahead of master
	ahead, errorAhead := checkDivergence(headRepoPath, fmt.Sprintf("%s/%s", tmpRemoteName, pr.BaseBranch), pr.HeadBranch)
	if errorAhead != nil {
		return &git.DivergeObject{}, errorAhead
	}

	// $(git rev-list --count feature..tmp-pull-1-base/master) commits behind master
	behind, errorBehind := checkDivergence(headRepoPath, pr.HeadBranch, fmt.Sprintf("%s/%s", tmpRemoteName, pr.BaseBranch))
	if errorBehind != nil {
		return &git.DivergeObject{}, errorBehind
	}

	return &git.DivergeObject{Ahead: ahead, Behind: behind}, nil
}

func checkDivergence(repoPath string, baseBranch string, targetBranch string) (int, error) {
	branches := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := git.NewCommand("rev-list", "--count", branches)
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return -1, err
	}
	outInteger, errInteger := strconv.Atoi(strings.Trim(stdout, "\n"))
	if errInteger != nil {
		return -1, errInteger
	}
	return outInteger, nil
}
