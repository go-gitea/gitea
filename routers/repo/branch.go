// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

const (
	tplBranchOverview = "repo/branches/overview"
	tplBranchAll      = "repo/branches/all"
)

type Branch struct {
	Name        string
	Commit      *git.Commit
	IsProtected bool
}

func loadBranches(c *context.Context) []*Branch {
	rawBranches, err := c.Repo.Repository.GetBranches()
	if err != nil {
		c.Handle(500, "GetBranches", err)
		return nil
	}

	protectBranches, err := c.Repo.Repository.GetProtectedBranches()
	if err != nil {
		c.Handle(500, "GetProtectBranchesByRepoID", err)
		return nil
	}

	branches := make([]*Branch, len(rawBranches))
	for i := range rawBranches {
		commit, err := rawBranches[i].GetCommit()
		if err != nil {
			c.Handle(500, "GetCommit", err)
			return nil
		}

		branches[i] = &Branch{
			Name:   rawBranches[i].Name,
			Commit: commit,
		}

		for j := range protectBranches {
			if branches[i].Name == protectBranches[j].BranchName {
				branches[i].IsProtected = true
				break
			}
		}
	}

	c.Data["AllowPullRequest"] = c.Repo.Repository.AllowsPulls()
	return branches
}

// Branches render repository branch page
func Branches(c *context.Context) {
	c.Data["Title"] = c.Tr("repo.git_branches")
	c.Data["PageIsBranchesOverview"] = true

	branches := loadBranches(c)
	if c.Written() {
		return
	}

	now := time.Now()
	activeBranches := make([]*Branch, 0, 3)
	staleBranches := make([]*Branch, 0, 3)
	for i := range branches {
		switch {
		case branches[i].Name == c.Repo.BranchName:
			c.Data["DefaultBranch"] = branches[i]
		case branches[i].Commit.Committer.When.Add(30 * 24 * time.Hour).After(now): // 30 days
			activeBranches = append(activeBranches, branches[i])
		case branches[i].Commit.Committer.When.Add(3 * 30 * 24 * time.Hour).Before(now): // 90 days
			staleBranches = append(staleBranches, branches[i])
		}
	}

	c.Data["ActiveBranches"] = activeBranches
	c.Data["StaleBranches"] = staleBranches
	c.HTML(200, tplBranchOverview)
}

// AllBranches all branches UI
func AllBranches(c *context.Context) {
	c.Data["Title"] = c.Tr("repo.git_branches")
	c.Data["PageIsBranchesAll"] = true

	branches := loadBranches(c)
	if c.Written() {
		return
	}
	c.Data["Branches"] = branches

	c.HTML(200, tplBranchAll)
}

// DeleteBranchPost executes branch deletation operation
func DeleteBranchPost(c *context.Context) {
	branchName := c.Params("*")
	commitID := c.Query("commit")

	defer func() {
		redirectTo := c.Query("redirect_to")
		if len(redirectTo) == 0 {
			redirectTo = c.Repo.RepoLink
		}
		c.Redirect(redirectTo)
	}()

	if !c.Repo.GitRepo.IsBranchExist(branchName) {
		return
	}
	if len(commitID) > 0 {
		branchCommitID, err := c.Repo.GitRepo.GetBranchCommitID(branchName)
		if err != nil {
			log.Error(2, "GetBranchCommitID: %v", err)
			return
		}

		if branchCommitID != commitID {
			c.Flash.Error(c.Tr("repo.pulls.delete_branch_has_new_commits"))
			return
		}
	}

	if err := c.Repo.GitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
		Force: true,
	}); err != nil {
		log.Error(2, "DeleteBranch '%s': %v", branchName, err)
		return
	}
}
