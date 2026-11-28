// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

// CodeActivityStats represents git statistics data
type CodeActivityStats struct {
	AuthorCount              int64
	CommitCount              int64
	ChangedFiles             int64
	Additions                int64
	Deletions                int64
	CommitCountInAllBranches int64
	Authors                  []*CodeActivityAuthor
}

// CodeActivityAuthor represents git statistics data for commit authors
type CodeActivityAuthor struct {
	Name    string
	Email   string
	Commits int64
}

type codeActivityCommit struct {
	firstParentID, authorName, authorEmail string
	isMerge                                bool
}

// GetCodeActivityStats returns code statistics for activity page
func (repo *Repository) GetCodeActivityStats(ctx context.Context, fromTime time.Time, branchName string) (*CodeActivityStats, error) {
	headCommitID, err := repo.GetBranchCommitID(ctx, branchName)
	if err != nil {
		return nil, err
	}
	stats := &CodeActivityStats{}
	commitsByID := make(map[string]codeActivityCommit)
	err = walkCodeActivityCommits(ctx, repo, fromTime, func(commitID string, commit codeActivityCommit) {
		commitsByID[commitID] = commit
		if !commit.isMerge {
			stats.CommitCountInAllBranches++
		}
	})
	if err != nil {
		return nil, err
	}

	authors := make(map[string]*CodeActivityAuthor)
	baseCommitID := headCommitID
	for commit, found := commitsByID[baseCommitID]; found; commit, found = commitsByID[baseCommitID] {
		stats.CommitCount += tallyCodeActivityCommit(authors, commit)
		baseCommitID = commit.firstParentID
	}
	stats.Authors, stats.AuthorCount = sortedCodeActivityAuthors(authors), int64(len(authors))
	if baseCommitID == headCommitID {
		return stats, nil
	}
	if baseCommitID == "" {
		baseCommitID = MustIDFromString(headCommitID).Type().EmptyTree().String()
	}
	diffStat, err := GetDiffShortStat(ctx, repo, baseCommitID, headCommitID, nil) // net change like GitHub's pulse, so renames and reverts don't add up per commit
	if err != nil {
		return nil, err
	}
	stats.ChangedFiles, stats.Additions, stats.Deletions = int64(diffStat.NumFiles), int64(diffStat.TotalAddition), int64(diffStat.TotalDeletion)
	return stats, nil
}

// GetCodeActivityAuthors returns the authors of non-merge commits in all branches since fromTime, sorted by commit count
func GetCodeActivityAuthors(ctx context.Context, repo RepositoryFacade, fromTime time.Time) ([]*CodeActivityAuthor, error) {
	authors := make(map[string]*CodeActivityAuthor)
	err := walkCodeActivityCommits(ctx, repo, fromTime, func(_commitID string, commit codeActivityCommit) {
		tallyCodeActivityCommit(authors, commit)
	})
	if err != nil {
		return nil, err
	}
	return sortedCodeActivityAuthors(authors), nil
}

func walkCodeActivityCommits(ctx context.Context, repo RepositoryFacade, fromTime time.Time, onCommit func(commitID string, commit codeActivityCommit)) error {
	cmd := gitcmd.NewCommand("log", "--branches=*", "--no-show-signature", "--pretty=tformat:%H%x00%P%x00%aN%x00%aE").
		AddOptionFormat("--since=%s", fromTime.Format(time.RFC3339)).
		WithRepo(repo)
	return gitcmd.RunWithScannerSplit(ctx, cmd, bufio.ScanLines, func(_ctx gitcmd.Context, line []byte) error {
		commitID, rest, _ := strings.Cut(string(line), "\x00")
		parentIDs, rest, _ := strings.Cut(rest, "\x00")
		name, email, _ := strings.Cut(rest, "\x00")
		firstParentID, _, isMerge := strings.Cut(parentIDs, " ")
		onCommit(commitID, codeActivityCommit{
			firstParentID: firstParentID, isMerge: isMerge,
			authorName: strings.TrimSpace(name), authorEmail: strings.ToLower(strings.TrimSpace(email)),
		})
		return nil
	})
}

func tallyCodeActivityCommit(authors map[string]*CodeActivityAuthor, commit codeActivityCommit) int64 {
	if commit.isMerge {
		return 0
	}
	if authors[commit.authorEmail] == nil {
		authors[commit.authorEmail] = &CodeActivityAuthor{Name: commit.authorName, Email: commit.authorEmail}
	}
	authors[commit.authorEmail].Commits++
	return 1
}

func sortedCodeActivityAuthors(authors map[string]*CodeActivityAuthor) []*CodeActivityAuthor {
	sorted := util.ValuesOfMap(authors)
	slices.SortFunc(sorted, func(first, second *CodeActivityAuthor) int { return cmp.Compare(second.Commits, first.Commits) })
	return sorted
}
