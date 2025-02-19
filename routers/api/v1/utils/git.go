// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	gocontext "context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
)

// ResolveRefOrSha resolve ref to sha if exist
func ResolveRefOrSha(ctx *context.APIContext, ref string) string {
	if len(ref) == 0 {
		ctx.APIError(http.StatusBadRequest, nil)
		return ""
	}

	sha := ref
	// Search branches and tags
	for _, refType := range []string{"heads", "tags"} {
		refSHA, lastMethodName, err := searchRefCommitByType(ctx, refType, ref)
		if err != nil {
			ctx.APIErrorInternal(fmt.Errorf("%s: %w", lastMethodName, err))
			return ""
		}
		if refSHA != "" {
			sha = refSHA
			break
		}
	}

	sha = MustConvertToSHA1(ctx, ctx.Repo, sha)

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

// ConvertToObjectID returns a full-length SHA1 from a potential ID string
func ConvertToObjectID(ctx gocontext.Context, repo *context.Repository, commitID string) (git.ObjectID, error) {
	objectFormat := repo.GetObjectFormat()
	if len(commitID) == objectFormat.FullLength() && objectFormat.IsValid(commitID) {
		sha, err := git.NewIDFromString(commitID)
		if err == nil {
			return sha, nil
		}
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo.Repository)
	if err != nil {
		return objectFormat.EmptyObjectID(), fmt.Errorf("RepositoryFromContextOrOpen: %w", err)
	}
	defer closer.Close()

	return gitRepo.ConvertToGitID(commitID)
}

// MustConvertToSHA1 returns a full-length SHA1 string from a potential ID string, or returns origin input if it can't convert to SHA1
func MustConvertToSHA1(ctx gocontext.Context, repo *context.Repository, commitID string) string {
	sha, err := ConvertToObjectID(ctx, repo, commitID)
	if err != nil {
		return commitID
	}
	return sha.String()
}
