// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	gocontext "context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// ResolveRefOrSha resolve ref to sha if exist
func ResolveRefOrSha(ctx *context.APIContext, ref string) string {
	if len(ref) == 0 {
		ctx.Error(http.StatusBadRequest, "ref not given", nil)
		return ""
	}

	sha := ref
	// Search branches and tags
	for _, refType := range []string{"heads", "tags"} {
		refSHA, lastMethodName, err := searchRefCommitByType(ctx, refType, ref)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, lastMethodName, err)
			return ""
		}
		if refSHA != "" {
			sha = refSHA
			break
		}
	}

	sha = MustConvertToHash(ctx, ctx.Repo, sha)

	if ctx.Repo.GitRepo != nil {
		err := ctx.Repo.GitRepo.AddLastCommitCache(ctx.Repo.Repository.GetCommitsCountCacheKey(ref, ref != sha), ctx.Repo.Repository.FullName(), sha)
		if err != nil {
			log.Error("Unable to get commits count for %s in %s. Error: %v", sha, ctx.Repo.Repository.FullName(), err)
		}
	}

	return sha
}

// GetGitRefs return git references based on filter
func GetGitRefs(ctx *context.APIContext, filter string) ([]*git.Reference, string, error) {
	if ctx.Repo.GitRepo == nil {
		return nil, "", fmt.Errorf("no open git repo found in context")
	}
	if len(filter) > 0 {
		filter = "refs/" + filter
	}
	refs, err := ctx.Repo.GitRepo.GetRefsFiltered(filter)
	return refs, "GetRefsFiltered", err
}

func searchRefCommitByType(ctx *context.APIContext, refType, filter string) (string, string, error) {
	refs, lastMethodName, err := GetGitRefs(ctx, refType+"/"+filter) // Search by type
	if err != nil {
		return "", lastMethodName, err
	}
	if len(refs) > 0 {
		return refs[0].Object.String(), "", nil // Return found SHA
	}
	return "", "", nil
}

// ConvertToSHA1 returns a full-length Hash from a potential ID string
func ConvertToHash(ctx gocontext.Context, repo *context.Repository, commitID string) (git.Hash, error) {
	if len(commitID) == repo.GitRepo.HashType.FullLength() && repo.GitRepo.HashType.IsValid(commitID) {
		sha1, err := git.NewIDFromString(commitID)
		if err == nil {
			return sha1, nil
		}
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.Repository.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("RepositoryFromContextOrOpen: %w", err)
	}
	defer closer.Close()

	return gitRepo.ConvertToHash(commitID)
}

// MustConvertToHash returns a full-length Hash string from a potential ID string, or returns origin input if it can't convert to Hash
func MustConvertToHash(ctx gocontext.Context, repo *context.Repository, commitID string) string {
	sha, err := ConvertToHash(ctx, repo, commitID)
	if err != nil {
		return commitID
	}
	return sha.String()
}
