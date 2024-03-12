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
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
)

func getDefaultBranchSha(ctx context.Context, repo *repo_model.Repository, isWiki bool) (string, error) {
	repoPath := repo.RepoPath()
	defaultBranch := repo.DefaultBranch
	if isWiki {
		repoPath = repo.WikiPath()
		defaultBranch = repo.DefaultWikiBranch
	}

	stdout, _, err := git.NewCommand(ctx, "show-ref", "-s").AddDynamicArguments(git.BranchPrefix + defaultBranch).RunStdString(&git.RunOpts{Dir: repoPath})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func getRepoStatus(ctx context.Context, repo *repo_model.Repository, isWiki bool) (*repo_model.RepoIndexerStatus, error) {
	indexerType := repo_model.RepoIndexerTypeCode
	if isWiki {
		indexerType = repo_model.RepoIndexerTypeWiki
	}

	return repo_model.GetIndexerStatus(ctx, repo, indexerType)
}

// getRepoChanges returns changes to repo since last indexer update
func getRepoChanges(ctx context.Context, repo *repo_model.Repository, isWiki bool, revision string) (*internal.RepoChanges, error) {
	repoPath := repo.RepoPath()
	if isWiki {
		repoPath = repo.WikiPath()
	}

	status, err := getRepoStatus(ctx, repo, isWiki)
	if err != nil {
		return nil, err
	}

	needGenesis := len(status.CommitSha) == 0
	if !needGenesis {
		hasAncestorCmd := git.NewCommand(ctx, "merge-base").AddDynamicArguments(status.CommitSha, revision)
		stdout, _, _ := hasAncestorCmd.RunStdString(&git.RunOpts{Dir: repoPath})
		needGenesis = len(stdout) == 0
	}

	if needGenesis {
		return genesisChanges(ctx, repo, isWiki, revision)
	}
	return nonGenesisChanges(ctx, repo, isWiki, status, revision)
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
func parseGitLsTreeOutput(objectFormat git.ObjectFormat, stdout []byte) ([]internal.FileUpdate, error) {
	entries, err := git.ParseTreeEntries(objectFormat, stdout)
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
func genesisChanges(ctx context.Context, repo *repo_model.Repository, isWiki bool, revision string) (*internal.RepoChanges, error) {
	var changes internal.RepoChanges
	repoPath := repo.RepoPath()
	if isWiki {
		repoPath = repo.WikiPath()
	}

	stdout, _, runErr := git.NewCommand(ctx, "ls-tree", "--full-tree", "-l", "-r").AddDynamicArguments(revision).RunStdBytes(&git.RunOpts{Dir: repoPath})
	if runErr != nil {
		return nil, runErr
	}

	repoObjectFormatName := "sha1"
	if !isWiki {
		repoObjectFormatName = repo.ObjectFormatName
	}
	objectFormat := git.ObjectFormatFromName(repoObjectFormatName)

	var err error
	changes.Updates, err = parseGitLsTreeOutput(objectFormat, stdout)
	return &changes, err
}

// nonGenesisChanges get changes since the previous indexer update
func nonGenesisChanges(ctx context.Context, repo *repo_model.Repository, isWiki bool, indexerStatus *repo_model.RepoIndexerStatus, revision string) (*internal.RepoChanges, error) {
	repoPath := repo.RepoPath()
	if isWiki {
		repoPath = repo.WikiPath()
	}

	diffCmd := git.NewCommand(ctx, "diff", "--name-status").AddDynamicArguments(indexerStatus.CommitSha, revision)
	stdout, _, runErr := diffCmd.RunStdString(&git.RunOpts{Dir: repoPath})
	if runErr != nil {
		// previous commit sha may have been removed by a force push, so
		// try rebuilding from scratch
		log.Warn("git diff: %v", runErr)
		if err := (*globalIndexer.Load()).Delete(ctx, repo.ID, optional.Some(isWiki)); err != nil {
			return nil, err
		}
		return genesisChanges(ctx, repo, isWiki, revision)
	}

	var changes internal.RepoChanges
	var err error
	updatedFilenames := make([]string, 0, 10)
	for _, line := range strings.Split(stdout, "\n") {
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
	}

	cmd := git.NewCommand(ctx, "ls-tree", "--full-tree", "-l").AddDynamicArguments(revision).
		AddDashesAndList(updatedFilenames...)
	lsTreeStdout, _, err := cmd.RunStdBytes(&git.RunOpts{Dir: repoPath})
	if err != nil {
		return nil, err
	}

	repoObjectFormatName := "sha1"
	if !isWiki {
		repoObjectFormatName = repo.ObjectFormatName
	}
	objectFormat := git.ObjectFormatFromName(repoObjectFormatName)

	changes.Updates, err = parseGitLsTreeOutput(objectFormat, lsTreeStdout)
	return &changes, err
}
