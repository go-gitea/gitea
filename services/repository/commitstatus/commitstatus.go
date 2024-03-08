// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

import (
	"context"
	"crypto/sha256"
	"fmt"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/automerge"
)

func getCacheKey(repoID int64, brancheName string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", repoID, brancheName)))
	return fmt.Sprintf("commit_status:%x", hashBytes)
}

func updateCommitStatusCache(ctx context.Context, repoID int64, branchName string, status api.CommitStatusState) error {
	c := cache.GetCache()
	return c.Put(getCacheKey(repoID, branchName), string(status), 3*24*60)
}

func deleteCommitStatusCache(ctx context.Context, repoID int64, branchName string) error {
	c := cache.GetCache()
	return c.Delete(getCacheKey(repoID, branchName))
}

// CreateCommitStatus creates a new CommitStatus given a bunch of parameters
// NOTE: All text-values will be trimmed from whitespaces.
// Requires: Repo, Creator, SHA
func CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, creator *user_model.User, sha string, status *git_model.CommitStatus) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %w", repoPath, err)
	}
	defer closer.Close()

	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)

	commit, err := gitRepo.GetCommit(sha)
	if err != nil {
		return fmt.Errorf("GetCommit[%s]: %w", sha, err)
	}
	if len(sha) != objectFormat.FullLength() {
		// use complete commit sha
		sha = commit.ID.String()
	}

	if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
		Repo:         repo,
		Creator:      creator,
		SHA:          commit.ID,
		CommitStatus: status,
	}); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
	}

	defaultBranchCommit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommit[%s]: %w", repo.DefaultBranch, err)
	}

	if commit.ID.String() == defaultBranchCommit.ID.String() { // since one commit status updated, the combined commit status should be invalid
		if err := deleteCommitStatusCache(ctx, repo.ID, repo.DefaultBranch); err != nil {
			log.Error("deleteCommitStatusCache[%d:%s] failed: %v", repo.ID, repo.DefaultBranch, err)
		}
	}

	if status.State.IsSuccess() {
		if err := automerge.MergeScheduledPullRequest(ctx, sha, repo); err != nil {
			return fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
		}
	}

	return nil
}

// FindReposLastestCommitStatuses loading repository default branch latest combinded commit status with cache
func FindReposLastestCommitStatuses(ctx context.Context, repos []*repo_model.Repository) ([]*git_model.CommitStatus, error) {
	results := make([]*git_model.CommitStatus, len(repos))
	c := cache.GetCache()

	for i, repo := range repos {
		status, ok := c.Get(getCacheKey(repo.ID, repo.DefaultBranch)).(string)
		if ok && status != "" {
			results[i] = &git_model.CommitStatus{State: api.CommitStatusState(status)}
		}
	}

	// collect the latest commit of each repo
	// at most there are dozens of repos (limited by MaxResponseItems), so it's not a big problem at the moment
	repoBranchNames := make(map[int64]string, len(repos))
	for i, repo := range repos {
		if results[i] == nil {
			repoBranchNames[repo.ID] = repo.DefaultBranch
		}
	}

	repoIDsToLatestCommitSHAs, err := git_model.FindBranchesByRepoAndBranchName(ctx, repoBranchNames)
	if err != nil {
		return nil, fmt.Errorf("FindBranchesByRepoAndBranchName: %v", err)
	}

	// call the database O(1) times to get the commit statuses for all repos
	repoToItsLatestCommitStatuses, err := git_model.GetLatestCommitStatusForPairs(ctx, repoIDsToLatestCommitSHAs, db.ListOptionsAll)
	if err != nil {
		return nil, fmt.Errorf("GetLatestCommitStatusForPairs: %v", err)
	}

	for i, repo := range repos {
		if results[i] == nil {
			results[i] = git_model.CalcCommitStatus(repoToItsLatestCommitStatuses[repo.ID])
			if results[i].State != "" {
				if err := updateCommitStatusCache(ctx, repo.ID, repo.DefaultBranch, results[i].State); err != nil {
					log.Error("updateCommitStatusCache[%d:%s] failed: %v", repo.ID, repo.DefaultBranch, err)
				}
			}
		}
	}

	return results, nil
}
