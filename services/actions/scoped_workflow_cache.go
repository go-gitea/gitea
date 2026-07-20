// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"

	lru "github.com/hashicorp/golang-lru/v2"
)

// cachedScopedWorkflows is one source repo's parsed scoped workflows together with the default-branch SHA they were parsed at.
type cachedScopedWorkflows struct {
	sha    string
	parsed []*actions_module.ParsedScopedWorkflow
}

// scopedWorkflowCache caches each scoped-workflow source repo's parsed workflows, keyed by source repo ID.
// There is exactly one entry per source: a default-branch update is detected by SHA mismatch and overwrites the entry, so stale parses never accumulate.
var scopedWorkflowCache *lru.Cache[int64, *cachedScopedWorkflows]

const defaultScopedWorkflowCacheSize = 1024

func init() {
	c, err := lru.New[int64, *cachedScopedWorkflows](defaultScopedWorkflowCacheSize)
	if err != nil {
		log.Fatal("failed to new scopedWorkflowCache, err: %v", err)
	}
	scopedWorkflowCache = c
}

// LoadParsedScopedWorkflows returns the source repo's parsed scoped workflows at its current default-branch HEAD.
func LoadParsedScopedWorkflows(ctx context.Context, sourceRepo *repo_model.Repository) (sha string, parsed []*actions_module.ParsedScopedWorkflow, err error) {
	branch, err := git_model.GetBranch(ctx, sourceRepo.ID, sourceRepo.DefaultBranch)
	if err != nil {
		return "", nil, fmt.Errorf("get source default branch: %w", err)
	}
	sha = branch.CommitID

	if v, ok := scopedWorkflowCache.Get(sourceRepo.ID); ok && v.sha == sha {
		// cache hit at the current default-branch HEAD
		return sha, v.parsed, nil
	}

	// cache miss: open the source repo at the exact SHA we keyed on
	sourceGitRepo, err := gitrepo.OpenRepository(sourceRepo)
	if err != nil {
		return "", nil, fmt.Errorf("open source repo: %w", err)
	}
	defer sourceGitRepo.Close()

	sourceCommit, err := sourceGitRepo.GetCommit(ctx, sha)
	if err != nil {
		return "", nil, fmt.Errorf("get source commit %s: %w", sha, err)
	}
	parsed, err = actions_module.ParseScopedWorkflows(ctx, sourceGitRepo, sourceCommit)
	if err != nil {
		return "", nil, err
	}
	// overwrite this source's single entry (a stale entry from a previous HEAD is replaced, not accumulated)
	scopedWorkflowCache.Add(sourceRepo.ID, &cachedScopedWorkflows{sha: sha, parsed: parsed})
	return sha, parsed, nil
}

// ScopedWorkflowContent returns one scoped workflow's raw content (by entry name) at the source repo's current default-branch HEAD, or nil if no such workflow exists there.
func ScopedWorkflowContent(ctx context.Context, sourceRepo *repo_model.Repository, entryName string) ([]byte, error) {
	_, parsed, err := LoadParsedScopedWorkflows(ctx, sourceRepo)
	if err != nil {
		return nil, err
	}
	for _, p := range parsed {
		if p.EntryName == entryName {
			return p.Content, nil
		}
	}
	return nil, nil
}
