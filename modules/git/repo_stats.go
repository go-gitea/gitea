// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/container"
	"gitea.dev/modules/git/gitcmd"
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

// GetCodeActivityStats returns code statistics for activity page
func (repo *Repository) GetCodeActivityStats(ctx context.Context, fromTime time.Time, branchName string) (*CodeActivityStats, error) {
	stdout, _, runErr := gitcmd.NewCommand("rev-list", "--count", "--no-merges", "--branches=*", "--date=iso").
		AddOptionFormat("--since=%s", fromTime.Format(time.RFC3339)).
		WithRepo(repo).
		RunStdString(ctx)
	if runErr != nil {
		return nil, runErr
	}

	commitCountInAllBranches, err := strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
	if err != nil {
		return nil, err
	}

	cmd := prepareCodeActivityLogCmd(fromTime, branchName).AddArguments("--numstat").WithRepo(repo)
	stats, err := parseCodeActivityLog(ctx, cmd)
	if err != nil {
		return nil, err
	}
	stats.CommitCountInAllBranches = commitCountInAllBranches
	return &stats, nil
}

// GetCodeActivityAuthors returns the authors of non-merge commits in all branches since fromTime, sorted by commit count
func GetCodeActivityAuthors(ctx context.Context, repo RepositoryFacade, fromTime time.Time) ([]*CodeActivityAuthor, error) {
	cmd := prepareCodeActivityLogCmd(fromTime, "").WithRepo(repo)
	stats, err := parseCodeActivityLog(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return stats.Authors, nil
}

const gitLogSep = ":::---:::" // should not conflict with any line of our git log output

func prepareCodeActivityLogCmd(fromTime time.Time, branchName string) *gitcmd.Command {
	cmd := gitcmd.NewCommand("log", "--no-merges", "--pretty=format:"+gitLogSep+"%n%h%n%aN%n%aE%n", "--date=iso").
		AddOptionFormat("--since=%s", fromTime.Format(time.RFC3339))
	if branchName == "" {
		cmd.AddArguments("--branches=*")
	} else {
		cmd.AddArguments("--first-parent").AddDynamicArguments(branchName)
	}
	return cmd
}

func parseCodeActivityLog(ctx context.Context, gitCmd *gitcmd.Command) (stats CodeActivityStats, _ error) {
	authors := make(map[string]*CodeActivityAuthor)
	files := make(container.Set[string])

	// output format is like this, see "man git-log" for details:
	//
	// ---
	// 1234567890
	// user-a
	// email-a@example.com
	//
	// 1       0       README.md
	//
	// ---
	// 1234abcd12
	// user-b
	// email-b@example.com
	// ...

	author := ""
	subIdx := 0
	err := gitcmd.RunWithScannerSplit(ctx, gitCmd, bufio.ScanLines, func(ctx gitcmd.Context, line []byte) error {
		if bytes.Equal(line, []byte(gitLogSep)) {
			subIdx = 1
			return nil
		} else if subIdx == 0 {
			return nil
		}

		subIdx++
		if subIdx > 4 && len(line) == 0 {
			return nil
		}

		switch subIdx {
		case 1: // Separator
		case 2: // Commit ID (short)
			stats.CommitCount++
		case 3: // Author
			author = string(line)
		case 4: // E-mail
			email := strings.ToLower(string(line))
			if _, ok := authors[email]; !ok {
				authors[email] = &CodeActivityAuthor{Name: author, Email: email, Commits: 0}
			}
			authors[email].Commits++
		default: // Changed files
			parts := bytes.SplitN(line, []byte{'\t'}, 3)
			if len(parts) != 3 {
				return nil
			}
			if add, _ := strconv.ParseInt(string(parts[0]), 10, 64); add > 0 {
				stats.Additions += add
			}
			if del, _ := strconv.ParseInt(string(parts[1]), 10, 64); del > 0 {
				stats.Deletions += del
			}
			filename := string(parts[2])
			filename, _ = UnquoteDiffFilePath(filename)
			files.Add(filename)
		}
		return nil
	})
	if err != nil {
		return stats, err
	}

	activityAuthors := make([]*CodeActivityAuthor, 0, len(authors))
	for _, v := range authors {
		activityAuthors = append(activityAuthors, v)
	}
	// Sort authors descending depending on commit count
	sort.Slice(activityAuthors, func(i, j int) bool { return activityAuthors[i].Commits > activityAuthors[j].Commits })
	stats.Authors = activityAuthors
	stats.AuthorCount = int64(len(authors))
	stats.ChangedFiles = int64(len(files))
	return stats, nil
}
