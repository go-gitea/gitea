// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
)

func isIndexable(entry *git.TreeEntry) bool {
	if !entry.IsRegular() && !entry.IsExecutable() {
		return false
	}
	name := strings.ToLower(entry.Name())
	for _, g := range setting.Indexer.ExcludePatterns {
		if g.Match(name) {
			return false
		}
	}
	for _, g := range setting.Indexer.IncludePatterns {
		if g.Match(name) {
			return true
		}
	}
	return len(setting.Indexer.IncludePatterns) == 0
}

// ParseGitLsTreeOutput parses the output of a `git ls-tree -r --full-name` command
func ParseGitLsTreeOutput(ctx context.Context, gitRepo *git.Repository, stdout []byte) ([]FileUpdate, error) {
	entries, err := git.ParseTreeEntries(stdout)
	if err != nil {
		return nil, err
	}
	idxCount := 0
	updates := make([]FileUpdate, len(entries))
	for _, entry := range entries {
		if isIndexable(entry) {
			updates[idxCount] = FileUpdate{
				Filename: entry.Name(),
				BlobSha:  entry.ID.String(),
				Size:     entry.GetSize(ctx, gitRepo),
				Sized:    true,
			}
			idxCount++
		}
	}
	return updates[:idxCount], nil
}

// GenesisChanges gets the changes needed to index the whole repository at the given revision
func GenesisChanges(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, revision string) (*RepoChanges, error) {
	changes := RepoChanges{Genesis: true}
	stdout, _, runErr := gitcmd.NewCommand("ls-tree", "--full-tree", "-l", "-r").AddDynamicArguments(revision).WithRepo(repo).RunStdBytes(ctx)
	if runErr != nil {
		return nil, runErr
	}

	var err error
	changes.Updates, err = ParseGitLsTreeOutput(ctx, gitRepo, stdout)
	return &changes, err
}
