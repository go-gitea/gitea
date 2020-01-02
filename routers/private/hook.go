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

	"gitea.com/macaron/macaron"
)

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *macaron.Context, opts private.HookOptions) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}
	repo.OwnerName = ownerName

	for i := range opts.OldCommitIDs {
		oldCommitID := opts.OldCommitIDs[i]
		newCommitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		branchName := strings.TrimPrefix(refFullName, git.BranchPrefix)
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
				if opts.GitAlternativeObjectDirectories != "" {
					env = append(env,
						private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
				}
				if opts.GitObjectDirectory != "" {
					env = append(env,
						private.GitObjectDirectory+"="+opts.GitObjectDirectory)
				}
				if opts.GitQuarantinePath != "" {
					env = append(env,
						private.GitQuarantinePath+"="+opts.GitQuarantinePath)
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
			canPush := false
			if opts.IsDeployKey {
				canPush = protectBranch.CanPush && (!protectBranch.EnableWhitelist || protectBranch.WhitelistDeployKeys)
			} else {
				canPush = protectBranch.CanUserPush(opts.UserID)
			}
			if !canPush && opts.ProtectedBranchID > 0 {
				pr, err := models.GetPullRequestByID(opts.ProtectedBranchID)
				if err != nil {
					log.Error("Unable to get PullRequest %d Error: %v", opts.ProtectedBranchID, err)
					ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
						"err": fmt.Sprintf("Unable to get PullRequest %d Error: %v", opts.ProtectedBranchID, err),
					})
					return
				}
				if !protectBranch.HasEnoughApprovals(pr) {
					log.Warn("Forbidden: User %d cannot push to protected branch: %s in %-v and pr #%d does not have enough approvals", opts.UserID, branchName, repo, pr.Index)
					ctx.JSON(http.StatusForbidden, map[string]interface{}{
						"err": fmt.Sprintf("protected branch %s can not be pushed to and pr #%d does not have enough approvals", branchName, opts.ProtectedBranchID),
					})
					return
				}
				if protectBranch.MergeBlockedByRejectedReview(pr) {
					log.Warn("Forbidden: User %d cannot push to protected branch: %s in %-v and pr #%d has requested changes", opts.UserID, branchName, repo, pr.Index)
					ctx.JSON(http.StatusForbidden, map[string]interface{}{
						"err": fmt.Sprintf("protected branch %s can not be pushed to and pr #%d has requested changes", branchName, opts.ProtectedBranchID),
					})
					return
				}
			} else if !canPush {
				log.Warn("Forbidden: User %d cannot push to protected branch: %s in %-v", opts.UserID, branchName, repo)
				ctx.JSON(http.StatusForbidden, map[string]interface{}{
					"err": fmt.Sprintf("protected branch %s can not be pushed to", branchName),
				})
				return
			}
		}
	}

	ctx.PlainText(http.StatusOK, []byte("ok"))
}

// HookPostReceive updates services and users
func HookPostReceive(ctx *macaron.Context, opts private.HookOptions) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")

	var repo *models.Repository
	updates := make([]*repofiles.PushUpdateOptions, 0, len(opts.OldCommitIDs))
	wasEmpty := false

	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		branch := opts.RefFullNames[i]
		if strings.HasPrefix(branch, git.BranchPrefix) {
			branch = strings.TrimPrefix(branch, git.BranchPrefix)
		} else {
			branch = strings.TrimPrefix(branch, git.TagPrefix)
		}

		// Only trigger activity updates for changes to branches or
		// tags.  Updates to other refs (eg, refs/notes, refs/changes,
		// or other less-standard refs spaces are ignored since there
		// may be a very large number of them).
		if strings.HasPrefix(refFullName, git.BranchPrefix) || strings.HasPrefix(refFullName, git.TagPrefix) {
			if repo == nil {
				var err error
				repo, err = models.GetRepositoryByOwnerAndName(ownerName, repoName)
				if err != nil {
					log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err: fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
					})
					return
				}
				if repo.OwnerName == "" {
					repo.OwnerName = ownerName
				}
				wasEmpty = repo.IsEmpty
			}

			option := repofiles.PushUpdateOptions{
				RefFullName:  refFullName,
				OldCommitID:  opts.OldCommitIDs[i],
				NewCommitID:  opts.NewCommitIDs[i],
				Branch:       branch,
				PusherID:     opts.UserID,
				PusherName:   opts.UserName,
				RepoUserName: ownerName,
				RepoName:     repoName,
			}
			updates = append(updates, &option)
			if repo.IsEmpty && branch == "master" && strings.HasPrefix(refFullName, git.BranchPrefix) {
				// put the master branch first
				copy(updates[1:], updates)
				updates[0] = &option
			}
		}
	}

	if repo != nil && len(updates) > 0 {
		if err := repofiles.PushUpdates(repo, updates); err != nil {
			log.Error("Failed to Update: %s/%s Total Updates: %d", ownerName, repoName, len(updates))
			for i, update := range updates {
				log.Error("Failed to Update: %s/%s Update: %d/%d: Branch: %s", ownerName, repoName, i, len(updates), update.Branch)
			}
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)

			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))

	// We have to reload the repo in case its state is changed above
	repo = nil
	var baseRepo *models.Repository

	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		newCommitID := opts.NewCommitIDs[i]

		branch := git.RefEndName(opts.RefFullNames[i])

		if newCommitID != git.EmptySHA && strings.HasPrefix(refFullName, git.BranchPrefix) {
			if repo == nil {
				var err error
				repo, err = models.GetRepositoryByOwnerAndName(ownerName, repoName)
				if err != nil {
					log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err:          fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
						RepoWasEmpty: wasEmpty,
					})
					return
				}
				if repo.OwnerName == "" {
					repo.OwnerName = ownerName
				}

				if !repo.AllowsPulls() {
					// We can stop there's no need to go any further
					ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
						RepoWasEmpty: wasEmpty,
					})
					return
				}
				baseRepo = repo

				if repo.IsFork {
					if err := repo.GetBaseRepo(); err != nil {
						log.Error("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err)
						ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
							Err:          fmt.Sprintf("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err),
							RepoWasEmpty: wasEmpty,
						})
						return
					}
					baseRepo = repo.BaseRepo
				}
			}

			if !repo.IsFork && branch == baseRepo.DefaultBranch {
				results = append(results, private.HookPostReceiveBranchResult{})
				continue
			}

			pr, err := models.GetUnmergedPullRequest(repo.ID, baseRepo.ID, branch, baseRepo.DefaultBranch)
			if err != nil && !models.IsErrPullRequestNotExist(err) {
				log.Error("Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf(
						"Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err),
					RepoWasEmpty: wasEmpty,
				})
				return
			}

			if pr == nil {
				if repo.IsFork {
					branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
				}
				results = append(results, private.HookPostReceiveBranchResult{
					Message: true,
					Create:  true,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/compare/%s...%s", baseRepo.HTMLURL(), util.PathEscapeSegments(baseRepo.DefaultBranch), util.PathEscapeSegments(branch)),
				})
			} else {
				results = append(results, private.HookPostReceiveBranchResult{
					Message: true,
					Create:  false,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/pulls/%d", baseRepo.HTMLURL(), pr.Index),
				})
			}
		}
	}
	ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
		Results:      results,
		RepoWasEmpty: wasEmpty,
	})
}

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *macaron.Context) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	branch := ctx.Params(":branch")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if repo.OwnerName == "" {
		repo.OwnerName = ownerName
	}

	repo.DefaultBranch = branch
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Failed to get git repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
		if !git.IsErrUnsupportedVersion(err) {
			gitRepo.Close()
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"Err": fmt.Sprintf("Unable to set default branch onrepository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}
	gitRepo.Close()

	if err := repo.UpdateDefaultBranch(); err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Unable to set default branch onrepository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	ctx.PlainText(200, []byte("success"))
}
