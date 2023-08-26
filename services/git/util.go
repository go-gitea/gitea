// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// ResolveRefOrSha resolve ref to sha if exist
func ResolveRefOrSha(ctx context.Context, repo *repo.Repository, gitRepo *git.Repository, ref string) (sha, lastMethodName string, err error) {

	sha = ref
	// Search branches and tags
	for _, refType := range []string{"heads", "tags"} {
		refSHA, lastMethodName, err := searchRefCommitByType(gitRepo, refType, ref)
		if err != nil {

			return "", lastMethodName, err
		}
		if refSHA != "" {
			sha = refSHA
			break
		}
	}

	sha = MustConvertToSHA1(ctx, repo, sha)

	if gitRepo != nil {
		err := gitRepo.AddLastCommitCache(repo.GetCommitsCountCacheKey(ref, ref != sha), repo.FullName(), sha)
		if err != nil {
			log.Error("Unable to get commits count for %s in %s. Error: %v", sha, repo.FullName(), err)
		}
	}

	return sha, "", nil
}

// GetGitRefs return git references based on filter
func GetGitRefs(gitRepo *git.Repository, filter string) ([]*git.Reference, string, error) {
	if gitRepo == nil {
		return nil, "", fmt.Errorf("no open git repo found")
	}
	if len(filter) > 0 {
		filter = "refs/" + filter
	}
	refs, err := gitRepo.GetRefsFiltered(filter)
	return refs, "GetRefsFiltered", err
}

func searchRefCommitByType(gitRepo *git.Repository, refType, filter string) (string, string, error) {
	refs, lastMethodName, err := GetGitRefs(gitRepo, refType+"/"+filter) // Search by type
	if err != nil {
		return "", lastMethodName, err
	}
	if len(refs) > 0 {
		return refs[0].Object.String(), "", nil // Return found SHA
	}
	return "", "", nil
}

// convertToSHA1 returns a full-length SHA1 from a potential ID string
func convertToSHA1(ctx context.Context, repo *repo.Repository, commitID string) (git.SHA1, error) {
	if len(commitID) == git.SHAFullLength && git.IsValidSHAPattern(commitID) {
		sha1, err := git.NewIDFromString(commitID)
		if err == nil {
			return sha1, nil
		}
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return git.SHA1{}, fmt.Errorf("RepositoryFromContextOrOpen: %w", err)
	}
	defer closer.Close()

	return gitRepo.ConvertToSHA1(commitID)
}

// MustConvertToSHA1 returns a full-length SHA1 string from a potential ID string, or returns origin input if it can't convert to SHA1
func MustConvertToSHA1(ctx context.Context, repo *repo.Repository, commitID string) string {
	sha, err := convertToSHA1(ctx, repo, commitID)
	if err != nil {
		return commitID
	}
	return sha.String()
}
