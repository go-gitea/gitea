// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
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
