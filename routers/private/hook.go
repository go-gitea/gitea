// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/repofiles"
	"code.gitea.io/gitea/modules/util"

	macaron "gopkg.in/macaron.v1"
)

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *macaron.Context) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	oldCommitID := ctx.QueryTrim("old")
	newCommitID := ctx.QueryTrim("new")
	refFullName := ctx.QueryTrim("ref")
	userID := ctx.QueryInt64("userID")
	gitObjectDirectory := ctx.QueryTrim("gitObjectDirectory")
	gitAlternativeObjectDirectories := ctx.QueryTrim("gitAlternativeObjectDirectories")
	gitQuarantinePath := ctx.QueryTrim("gitQuarantinePath")
	prID := ctx.QueryInt64("prID")

	branchName := strings.TrimPrefix(refFullName, git.BranchPrefix)
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	repo.OwnerName = ownerName
	protectBranch, err := models.GetProtectedBranchBy(repo.ID, branchName)
	if err != nil {
		log.Error("Unable to get protected branch: %s in %-v Error: %v", branchName, repo, err)
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	if protectBranch != nil && protectBranch.IsProtected() {
		// check and deletion
		if newCommitID == git.EmptySHA {
			log.Warn("Forbidden: Branch: %s in %-v is protected from deletion", branchName, repo)
			ctx.JSON(http.StatusForbidden, map[string]interface{}{
				"err": fmt.Sprintf("branch %s is protected from deletion", branchName),
			})
			return
		}

		// detect force push
		if git.EmptySHA != oldCommitID {
			env := os.Environ()
			if gitAlternativeObjectDirectories != "" {
				env = append(env,
					private.GitAlternativeObjectDirectories+"="+gitAlternativeObjectDirectories)
			}
			if gitObjectDirectory != "" {
				env = append(env,
					private.GitObjectDirectory+"="+gitObjectDirectory)
			}
			if gitQuarantinePath != "" {
				env = append(env,
					private.GitQuarantinePath+"="+gitQuarantinePath)
			}

			output, err := git.NewCommand("rev-list", "--max-count=1", oldCommitID, "^"+newCommitID).RunInDirWithEnv(repo.RepoPath(), env)
			if err != nil {
				log.Error("Unable to detect force push between: %s and %s in %-v Error: %v", oldCommitID, newCommitID, repo, err)
				ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
					"err": fmt.Sprintf("Fail to detect force push: %v", err),
				})
				return
			} else if len(output) > 0 {
				log.Warn("Forbidden: Branch: %s in %-v is protected from force push", branchName, repo)
				ctx.JSON(http.StatusForbidden, map[string]interface{}{
					"err": fmt.Sprintf("branch %s is protected from force push", branchName),
				})
				return

			}
		}

		canPush := protectBranch.CanUserPush(userID)
		if !canPush && prID > 0 {
			pr, err := models.GetPullRequestByID(prID)
			if err != nil {
				log.Error("Unable to get PullRequest %d Error: %v", prID, err)
				ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
					"err": fmt.Sprintf("Unable to get PullRequest %d Error: %v", prID, err),
				})
				return
			}
			if !protectBranch.HasEnoughApprovals(pr) {
				log.Warn("Forbidden: User %d cannot push to protected branch: %s in %-v and pr #%d does not have enough approvals", userID, branchName, repo, pr.Index)
				ctx.JSON(http.StatusForbidden, map[string]interface{}{
					"err": fmt.Sprintf("protected branch %s can not be pushed to and pr #%d does not have enough approvals", branchName, prID),
				})
				return
			}
		} else if !canPush {
			log.Warn("Forbidden: User %d cannot push to protected branch: %s in %-v", userID, branchName, repo)
			ctx.JSON(http.StatusForbidden, map[string]interface{}{
				"err": fmt.Sprintf("protected branch %s can not be pushed to", branchName),
			})
			return
		}
	}
	ctx.PlainText(http.StatusOK, []byte("ok"))
}

// HookPostReceive updates services and users
func HookPostReceive(ctx *macaron.Context) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	oldCommitID := ctx.Query("old")
	newCommitID := ctx.Query("new")
	refFullName := ctx.Query("ref")
	userID := ctx.QueryInt64("userID")
	userName := ctx.Query("username")

	branch := refFullName
	if strings.HasPrefix(refFullName, git.BranchPrefix) {
		branch = strings.TrimPrefix(refFullName, git.BranchPrefix)
	} else if strings.HasPrefix(refFullName, git.TagPrefix) {
		branch = strings.TrimPrefix(refFullName, git.TagPrefix)
	}

	// Only trigger activity updates for changes to branches or
	// tags.  Updates to other refs (eg, refs/notes, refs/changes,
	// or other less-standard refs spaces are ignored since there
	// may be a very large number of them).
	if strings.HasPrefix(refFullName, git.BranchPrefix) || strings.HasPrefix(refFullName, git.TagPrefix) {
		repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
		if err != nil {
			log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
		if err := repofiles.PushUpdate(repo, branch, models.PushUpdateOptions{
			RefFullName:  refFullName,
			OldCommitID:  oldCommitID,
			NewCommitID:  newCommitID,
			PusherID:     userID,
			PusherName:   userName,
			RepoUserName: ownerName,
			RepoName:     repoName,
		}); err != nil {
			log.Error("Failed to Update: %s/%s Branch: %s Error: %v", ownerName, repoName, branch, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": fmt.Sprintf("Failed to Update: %s/%s Branch: %s Error: %v", ownerName, repoName, branch, err),
			})
			return
		}
	}

	if newCommitID != git.EmptySHA && strings.HasPrefix(refFullName, git.BranchPrefix) {
		repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
		if err != nil {
			log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
		repo.OwnerName = ownerName

		pullRequestAllowed := repo.AllowsPulls()
		if !pullRequestAllowed {
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"message": false,
			})
			return
		}

		baseRepo := repo
		if repo.IsFork {
			if err := repo.GetBaseRepo(); err != nil {
				log.Error("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err)
				ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
					"err": fmt.Sprintf("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err),
				})
				return
			}
			baseRepo = repo.BaseRepo
		}

		if !repo.IsFork && branch == baseRepo.DefaultBranch {
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"message": false,
			})
			return
		}

		pr, err := models.GetUnmergedPullRequest(repo.ID, baseRepo.ID, branch, baseRepo.DefaultBranch)
		if err != nil && !models.IsErrPullRequestNotExist(err) {
			log.Error("Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err)
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": fmt.Sprintf(
					"Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err),
			})
			return
		}

		if pr == nil {
			if repo.IsFork {
				branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
			}
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"message": true,
				"create":  true,
				"branch":  branch,
				"url":     fmt.Sprintf("%s/compare/%s...%s", baseRepo.HTMLURL(), util.PathEscapeSegments(baseRepo.DefaultBranch), util.PathEscapeSegments(branch)),
			})
		} else {
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"message": true,
				"create":  false,
				"branch":  branch,
				"url":     fmt.Sprintf("%s/pulls/%d", baseRepo.HTMLURL(), pr.Index),
			})
		}
		return
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"message": false,
	})
}
