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

// Branch branch information on UI
type Branch struct {
	Name        string
	Commit      *git.Commit
	IsProtected bool
}

func loadBranches(ctx *context.Context) []*Branch {
	rawBranches, err := ctx.Repo.Repository.GetBranches()
	if err != nil {
		ctx.Handle(500, "GetBranches", err)
		return nil
	}

	protectBranches, err := ctx.Repo.Repository.GetProtectedBranches()
	if err != nil {
		ctx.Handle(500, "GetProtectedBranches", err)
		return nil
	}

	branches := make([]*Branch, len(rawBranches))
	for i := range rawBranches {
		commit, err := rawBranches[i].GetCommit()
		if err != nil {
			ctx.Handle(500, "GetCommit", err)
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

	ctx.Data["AllowPullRequest"] = ctx.Repo.Repository.AllowsPulls()
	return branches
}

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.git_branches")
	ctx.Data["PageIsBranchesOverview"] = true

	branches := loadBranches(ctx)
	if ctx.Written() {
		return
	}

	now := time.Now()
	activeBranches := make([]*Branch, 0, 3)
	staleBranches := make([]*Branch, 0, 3)
	for i := range branches {
		switch {
		case branches[i].Name == ctx.Repo.BranchName:
			ctx.Data["DefaultBranch"] = branches[i]
		case branches[i].Commit.Committer.When.Add(30 * 24 * time.Hour).After(now): // 30 days
			activeBranches = append(activeBranches, branches[i])
		case branches[i].Commit.Committer.When.Add(3 * 30 * 24 * time.Hour).Before(now): // 90 days
			staleBranches = append(staleBranches, branches[i])
		}
	}

	ctx.Data["ActiveBranches"] = activeBranches
	ctx.Data["StaleBranches"] = staleBranches
	ctx.HTML(200, tplBranchOverview)
}

// AllBranches all branches UI
func AllBranches(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.git_branches")
	ctx.Data["PageIsBranchesAll"] = true

	branches := loadBranches(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Branches"] = branches

	ctx.HTML(200, tplBranchAll)
}

// DeleteBranchPost executes branch deletation operation
func DeleteBranchPost(ctx *context.Context) {
	branchName := ctx.Params("*")
	commitID := ctx.Query("commit")

	defer func() {
		redirectTo := ctx.Query("redirect_to")
		if len(redirectTo) == 0 {
			redirectTo = ctx.Repo.RepoLink
		}
		ctx.Redirect(redirectTo)
	}()

	if !ctx.Repo.GitRepo.IsBranchExist(branchName) {
		return
	}
	if len(commitID) > 0 {
		branchCommitID, err := ctx.Repo.GitRepo.GetBranchCommitID(branchName)
		if err != nil {
			log.Error(2, "GetBranchCommitID: %v", err)
			return
		}

		if branchCommitID != commitID {
			ctx.Flash.Error(ctx.Tr("repo.pulls.delete_branch_has_new_commits"))
			return
		}
	}

	if err := ctx.Repo.GitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
		Force: true,
	}); err != nil {
		log.Error(2, "DeleteBranch '%s': %v", branchName, err)
		return
	}
}
