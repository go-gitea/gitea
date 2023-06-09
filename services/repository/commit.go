// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/util"
	gitea_ctx "code.gitea.io/gitea/modules/context"
)

type containedLinks struct { // TODO: better name?
	Branches []*namedLink `json:"branches"`
	Tags []*namedLink `json:"tags"`
}

type namedLink struct { // TODO: better name?
	Name string `json:"name"`
	WebLink string `json:"web_url"`
}

// CreateNewBranch creates a new repository branch
func LoadBranchesAndTags(ctx context.Context, baseRepo *gitea_ctx.Repository, commitSHA string) (*containedLinks, error) {
	containedTags, err := baseRepo.GitRepo.ListOccurrences(ctx, "tag", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("encountered a problem while querying %s: %w", "tags", err)
	}
	containedBranches, err := baseRepo.GitRepo.ListOccurrences(ctx, "branch", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("encountered a problem while querying %s: %w", "branches", err)
	}

	result := &containedLinks{Branches: make([]*namedLink, 0, len(containedBranches)), Tags: make([]*namedLink, 0, len(containedTags))}
	for _, tag := range containedTags {
		result.Tags = append(result.Tags, &namedLink{Name: tag, WebLink: fmt.Sprintf("%s/src/tag/%s", baseRepo.RepoLink, util.PathEscapeSegments(tag))}) // TODO: Use a common method to get the link to a branch/tag instead of hardcoding it here
	}
	for _, branch := range containedBranches {
		result.Branches = append(result.Branches, &namedLink{Name: branch, WebLink: fmt.Sprintf("%s/src/branch/%s", baseRepo.RepoLink, util.PathEscapeSegments(branch))})
	}

	return result, nil
}
