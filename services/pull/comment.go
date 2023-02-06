// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	issue_service "code.gitea.io/gitea/services/issue"
)

type commitBranchCheckItem struct {
	Commit  *git.Commit
	Checked bool
}

func commitBranchCheck(gitRepo *git.Repository, startCommit *git.Commit, endCommitID, baseBranch string, commitList map[string]*commitBranchCheckItem) error {
	if startCommit.ID.String() == endCommitID {
		return nil
	}

	checkStack := make([]string, 0, 10)
	checkStack = append(checkStack, startCommit.ID.String())

	for len(checkStack) > 0 {
		commitID := checkStack[0]
		checkStack = checkStack[1:]

		item, ok := commitList[commitID]
		if !ok {
			continue
		}

		if item.Commit.ID.String() == endCommitID {
			continue
		}

		if err := item.Commit.LoadBranchName(); err != nil {
			return err
		}

		if item.Commit.Branch == baseBranch {
			continue
		}

		if item.Checked {
			continue
		}

		item.Checked = true

		parentNum := item.Commit.ParentCount()
		for i := 0; i < parentNum; i++ {
			parentCommit, err := item.Commit.Parent(i)
			if err != nil {
				return err
			}
			checkStack = append(checkStack, parentCommit.ID.String())
		}
	}
	return nil
}

// getCommitIDsFromRepo get commit IDs from repo in between oldCommitID and newCommitID
// isForcePush will be true if oldCommit isn't on the branch
// Commit on baseBranch will skip
func getCommitIDsFromRepo(ctx context.Context, repo *repo_model.Repository, oldCommitID, newCommitID, baseBranch string) (commitIDs []string, isForcePush bool, err error) {
	repoPath := repo.RepoPath()
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repoPath)
	if err != nil {
		return nil, false, err
	}
	defer closer.Close()

	oldCommit, err := gitRepo.GetCommit(oldCommitID)
	if err != nil {
		return nil, false, err
	}

	if err = oldCommit.LoadBranchName(); err != nil {
		return nil, false, err
	}

	if len(oldCommit.Branch) == 0 {
		commitIDs = make([]string, 2)
		commitIDs[0] = oldCommitID
		commitIDs[1] = newCommitID

		return commitIDs, true, err
	}

	newCommit, err := gitRepo.GetCommit(newCommitID)
	if err != nil {
		return nil, false, err
	}

	commits, err := newCommit.CommitsBeforeUntil(oldCommitID)
	if err != nil {
		return nil, false, err
	}

	commitIDs = make([]string, 0, len(commits))
	commitChecks := make(map[string]*commitBranchCheckItem)

	for _, commit := range commits {
		commitChecks[commit.ID.String()] = &commitBranchCheckItem{
			Commit:  commit,
			Checked: false,
		}
	}

	if err = commitBranchCheck(gitRepo, newCommit, oldCommitID, baseBranch, commitChecks); err != nil {
		return
	}

	for i := len(commits) - 1; i >= 0; i-- {
		commitID := commits[i].ID.String()
		if item, ok := commitChecks[commitID]; ok && item.Checked {
			commitIDs = append(commitIDs, commitID)
		}
	}

	return commitIDs, isForcePush, err
}

// CreatePushPullComment create push code to pull base comment
func CreatePushPullComment(ctx context.Context, pusher *user_model.User, pr *issues_model.PullRequest, oldCommitID, newCommitID string) (comment *issues_model.Comment, err error) {
	if pr.HasMerged || oldCommitID == "" || newCommitID == "" {
		return nil, nil
	}

	ops := &issues_model.CreateCommentOptions{
		Type: issues_model.CommentTypePullRequestPush,
		Doer: pusher,
		Repo: pr.BaseRepo,
	}

	var data issues_model.PushActionContent

	data.CommitIDs, data.IsForcePush, err = getCommitIDsFromRepo(ctx, pr.BaseRepo, oldCommitID, newCommitID, pr.BaseBranch)
	if err != nil {
		return nil, err
	}

	ops.Issue = pr.Issue

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ops.Content = string(dataJSON)

	comment, err = issue_service.CreateComment(ops)

	return comment, err
}
