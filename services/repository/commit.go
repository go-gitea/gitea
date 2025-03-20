// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

type ContainedLinks struct { // TODO: better name?
	Branches      []*namedLink `json:"branches"`
	Tags          []*namedLink `json:"tags"`
	DefaultBranch string       `json:"default_branch"`
}

type namedLink struct { // TODO: better name?
	Name    string `json:"name"`
	WebLink string `json:"web_link"`
}

// LoadBranchesAndTags creates a new repository branch
func LoadBranchesAndTags(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, repoLink string, commitSHA string) (*ContainedLinks, error) {
	containedTags, err := gitRepo.ListOccurrences(ctx, "tag", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("encountered a problem while querying %s: %w", "tags", err)
	}
	containedBranches, err := gitRepo.ListOccurrences(ctx, "branch", commitSHA)
	if err != nil {
		return nil, fmt.Errorf("encountered a problem while querying %s: %w", "branches", err)
	}

	result := &ContainedLinks{
		DefaultBranch: repo.DefaultBranch,
		Branches:      make([]*namedLink, 0, len(containedBranches)),
		Tags:          make([]*namedLink, 0, len(containedTags)),
	}
	for _, tag := range containedTags {
		// TODO: Use a common method to get the link to a branch/tag instead of hard-coding it here
		result.Tags = append(result.Tags, &namedLink{
			Name:    tag,
			WebLink: fmt.Sprintf("%s/src/tag/%s", repoLink, util.PathEscapeSegments(tag)),
		})
	}
	for _, branch := range containedBranches {
		result.Branches = append(result.Branches, &namedLink{
			Name:    branch,
			WebLink: fmt.Sprintf("%s/src/branch/%s", repoLink, util.PathEscapeSegments(branch)),
		})
	}
	return result, nil
}
