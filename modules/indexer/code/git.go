// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func getDefaultBranchSha(ctx context.Context, repo *repo_model.Repository) (string, error) {
	stdout, _, err := git.NewCommand("show-ref", "-s").AddDynamicArguments(git.BranchPrefix+repo.DefaultBranch).RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// getRepoChanges returns changes to repo since last indexer update
func getRepoChanges(ctx context.Context, repo *repo_model.Repository, revision string) (*internal.RepoChanges, error) {
	status, err := repo_model.GetIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeCode)
	if err != nil {
		return nil, err
	}

	needGenesis := len(status.CommitSha) == 0
	if !needGenesis {
		hasAncestorCmd := git.NewCommand("merge-base").AddDynamicArguments(status.CommitSha, revision)
		stdout, _, _ := hasAncestorCmd.RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
		needGenesis = len(stdout) == 0
	}

	if needGenesis {
		return genesisChanges(ctx, repo, revision)
	}
	return nonGenesisChanges(ctx, repo, revision)
}

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

// parseGitLsTreeOutput parses the output of a `git ls-tree -r --full-name` command
func parseGitLsTreeOutput(stdout []byte) ([]internal.FileUpdate, error) {
	entries, err := git.ParseTreeEntries(stdout)
	if err != nil {
		return nil, err
	}
	idxCount := 0
	updates := make([]internal.FileUpdate, len(entries))
	for _, entry := range entries {
		if isIndexable(entry) {
			updates[idxCount] = internal.FileUpdate{
				Filename: entry.Name(),
				BlobSha:  entry.ID.String(),
				Size:     entry.Size(),
				Sized:    true,
			}
			idxCount++
		}
	}
	return updates[:idxCount], nil
}

// genesisChanges get changes to add repo to the indexer for the first time
func genesisChanges(ctx context.Context, repo *repo_model.Repository, revision string) (*internal.RepoChanges, error) {
	var changes internal.RepoChanges
	stdout, _, runErr := git.NewCommand("ls-tree", "--full-tree", "-l", "-r").AddDynamicArguments(revision).RunStdBytes(ctx, &git.RunOpts{Dir: repo.RepoPath()})
	if runErr != nil {
		return nil, runErr
	}

	var err error
	changes.Updates, err = parseGitLsTreeOutput(stdout)
	return &changes, err
}

// nonGenesisChanges get changes since the previous indexer update
func nonGenesisChanges(ctx context.Context, repo *repo_model.Repository, revision string) (*internal.RepoChanges, error) {
	diffCmd := git.NewCommand("diff", "--name-status").AddDynamicArguments(repo.CodeIndexerStatus.CommitSha, revision)
	stdout, _, runErr := diffCmd.RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
	if runErr != nil {
		// previous commit sha may have been removed by a force push, so
		// try rebuilding from scratch
		log.Warn("git diff: %v", runErr)
		if err := (*globalIndexer.Load()).Delete(ctx, repo.ID); err != nil {
			return nil, err
		}
		return genesisChanges(ctx, repo, revision)
	}

	var changes internal.RepoChanges
	var err error
	updatedFilenames := make([]string, 0, 10)

	updateChanges := func() error {
		cmd := git.NewCommand("ls-tree", "--full-tree", "-l").AddDynamicArguments(revision).
			AddDashesAndList(updatedFilenames...)
		lsTreeStdout, _, err := cmd.RunStdBytes(ctx, &git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return err
		}

		updates, err1 := parseGitLsTreeOutput(lsTreeStdout)
		if err1 != nil {
			return err1
		}
		changes.Updates = append(changes.Updates, updates...)
		return nil
	}
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			log.Warn("Unparseable output for diff --name-status: `%s`)", line)
			continue
		}
		filename := fields[1]
		if len(filename) == 0 {
			continue
		} else if filename[0] == '"' {
			filename, err = strconv.Unquote(filename)
			if err != nil {
				return nil, err
			}
		}

		switch status := fields[0][0]; status {
		case 'M', 'A':
			updatedFilenames = append(updatedFilenames, filename)
		case 'D':
			changes.RemovedFilenames = append(changes.RemovedFilenames, filename)
		case 'R', 'C':
			if len(fields) < 3 {
				log.Warn("Unparseable output for diff --name-status: `%s`)", line)
				continue
			}
			dest := fields[2]
			if len(dest) == 0 {
				log.Warn("Unparseable output for diff --name-status: `%s`)", line)
				continue
			}
			if dest[0] == '"' {
				dest, err = strconv.Unquote(dest)
				if err != nil {
					return nil, err
				}
			}
			if status == 'R' {
				changes.RemovedFilenames = append(changes.RemovedFilenames, filename)
			}
			updatedFilenames = append(updatedFilenames, dest)
		default:
			log.Warn("Unrecognized status: %c (line=%s)", status, line)
		}

		// According to https://learn.microsoft.com/en-us/troubleshoot/windows-client/shell-experience/command-line-string-limitation#more-information
		// the command line length should less than 8191 characters, assume filepath is 256, then 8191/256 = 31, so we use 30
		if len(updatedFilenames) >= 30 {
			if err := updateChanges(); err != nil {
				return nil, err
			}
			updatedFilenames = updatedFilenames[0:0]
		}
	}

	if len(updatedFilenames) > 0 {
		if err := updateChanges(); err != nil {
			return nil, err
		}
	}

	return &changes, err
}
