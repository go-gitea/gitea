// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/automerge"
)

func getCacheKey(repoID int64, brancheName string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", repoID, brancheName)))
	return fmt.Sprintf("commit_status:%x", hashBytes)
}

type commitStatusCacheValue struct {
	State     string `json:"state"`
	TargetURL string `json:"target_url"`
}

func getCommitStatusCache(repoID int64, branchName string) *commitStatusCacheValue {
	c := cache.GetCache()
	statusStr, ok := c.Get(getCacheKey(repoID, branchName))
	if ok && statusStr != "" {
		var cv commitStatusCacheValue
		err := json.Unmarshal([]byte(statusStr), &cv)
		if err == nil {
			return &cv
		}
		log.Warn("getCommitStatusCache: json.Unmarshal failed: %v", err)
	}
	return nil
}

func updateCommitStatusCache(repoID int64, branchName string, state api.CommitStatusState, targetURL string) error {
	c := cache.GetCache()
	bs, err := json.Marshal(commitStatusCacheValue{
		State:     state.String(),
		TargetURL: targetURL,
	})
	if err != nil {
		log.Warn("updateCommitStatusCache: json.Marshal failed: %v", err)
		return nil
	}
	return c.Put(getCacheKey(repoID, branchName), string(bs), 3*24*60)
}

func deleteCommitStatusCache(repoID int64, branchName string) error {
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

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
			Repo:         repo,
			Creator:      creator,
			SHA:          commit.ID,
			CommitStatus: status,
		}); err != nil {
			return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
		}

		return git_model.UpdateCommitStatusSummary(ctx, repo.ID, commit.ID.String())
	}); err != nil {
		return err
	}

	defaultBranchCommit, err := gitRepo.GetBranchCommit(repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommit[%s]: %w", repo.DefaultBranch, err)
	}

	if commit.ID.String() == defaultBranchCommit.ID.String() { // since one commit status updated, the combined commit status should be invalid
		if err := deleteCommitStatusCache(repo.ID, repo.DefaultBranch); err != nil {
			log.Error("deleteCommitStatusCache[%d:%s] failed: %v", repo.ID, repo.DefaultBranch, err)
		}
	}

	if status.State.IsSuccess() {
		if err := automerge.StartPRCheckAndAutoMergeBySHA(ctx, sha, repo); err != nil {
			return fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
		}
	}

	return nil
}

// FindReposLastestCommitStatuses loading repository default branch latest combinded commit status with cache
func FindReposLastestCommitStatuses(ctx context.Context, repos []*repo_model.Repository) ([]*git_model.CommitStatus, error) {
	results := make([]*git_model.CommitStatus, len(repos))
	allCached := true
	for i, repo := range repos {
		if cv := getCommitStatusCache(repo.ID, repo.DefaultBranch); cv != nil {
			results[i] = &git_model.CommitStatus{
				State:     api.CommitStatusState(cv.State),
				TargetURL: cv.TargetURL,
			}
		} else {
			allCached = false
		}
	}

	if allCached {
		return results, nil
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

	var repoSHAs []git_model.RepoSHA
	for id, sha := range repoIDsToLatestCommitSHAs {
		repoSHAs = append(repoSHAs, git_model.RepoSHA{RepoID: id, SHA: sha})
	}

	summaryResults, err := git_model.GetLatestCommitStatusForRepoAndSHAs(ctx, repoSHAs)
	if err != nil {
		return nil, fmt.Errorf("GetLatestCommitStatusForRepoAndSHAs: %v", err)
	}

	for _, summary := range summaryResults {
		for i, repo := range repos {
			if repo.ID == summary.RepoID {
				results[i] = summary
				repoSHAs = slices.DeleteFunc(repoSHAs, func(repoSHA git_model.RepoSHA) bool {
					return repoSHA.RepoID == repo.ID
				})
				if results[i] != nil {
					if err := updateCommitStatusCache(repo.ID, repo.DefaultBranch, results[i].State, results[i].TargetURL); err != nil {
						log.Error("updateCommitStatusCache[%d:%s] failed: %v", repo.ID, repo.DefaultBranch, err)
					}
				}
				break
			}
		}
	}
	if len(repoSHAs) == 0 {
		return results, nil
	}

	// call the database O(1) times to get the commit statuses for all repos
	repoToItsLatestCommitStatuses, err := git_model.GetLatestCommitStatusForPairs(ctx, repoSHAs)
	if err != nil {
		return nil, fmt.Errorf("GetLatestCommitStatusForPairs: %v", err)
	}

	for i, repo := range repos {
		if results[i] == nil {
			results[i] = git_model.CalcCommitStatus(repoToItsLatestCommitStatuses[repo.ID])
			if results[i] != nil {
				if err := updateCommitStatusCache(repo.ID, repo.DefaultBranch, results[i].State, results[i].TargetURL); err != nil {
					log.Error("updateCommitStatusCache[%d:%s] failed: %v", repo.ID, repo.DefaultBranch, err)
				}
			}
		}
	}

	return results, nil
}
