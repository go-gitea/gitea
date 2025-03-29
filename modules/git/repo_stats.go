// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/container"
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
func (repo *Repository) GetCodeActivityStats(fromTime time.Time, branch string) (*CodeActivityStats, error) {
	stats := &CodeActivityStats{}

	since := fromTime.Format(time.RFC3339)

	stdout, _, runErr := NewCommand("rev-list", "--count", "--no-merges", "--branches=*", "--date=iso").AddOptionFormat("--since='%s'", since).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}

	c, err := strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
	if err != nil {
		return nil, err
	}
	stats.CommitCountInAllBranches = c

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	gitCmd := NewCommand("log", "--numstat", "--no-merges", "--pretty=format:---%n%h%n%aN%n%aE%n", "--date=iso").AddOptionFormat("--since='%s'", since)
	if len(branch) == 0 {
		gitCmd.AddArguments("--branches=*")
	} else {
		gitCmd.AddArguments("--first-parent").AddDynamicArguments(branch)
	}

	stderr := new(strings.Builder)
	err = gitCmd.Run(repo.Ctx, &RunOpts{
		Env:    []string{},
		Dir:    repo.Path,
		Stdout: stdoutWriter,
		Stderr: stderr,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			scanner := bufio.NewScanner(stdoutReader)
			scanner.Split(bufio.ScanLines)
			stats.CommitCount = 0
			stats.Additions = 0
			stats.Deletions = 0
			authors := make(map[string]*CodeActivityAuthor)
			files := make(container.Set[string])
			var author string
			p := 0
			for scanner.Scan() {
				l := strings.TrimSpace(scanner.Text())
				if l == "---" {
					p = 1
				} else if p == 0 {
					continue
				} else {
					p++
				}
				if p > 4 && len(l) == 0 {
					continue
				}
				switch p {
				case 1: // Separator
				case 2: // Commit sha-1
					stats.CommitCount++
				case 3: // Author
					author = l
				case 4: // E-mail
					email := strings.ToLower(l)
					if _, ok := authors[email]; !ok {
						authors[email] = &CodeActivityAuthor{Name: author, Email: email, Commits: 0}
					}
					authors[email].Commits++
				default: // Changed file
					if parts := strings.Fields(l); len(parts) >= 3 {
						if parts[0] != "-" {
							if c, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64); err == nil {
								stats.Additions += c
							}
						}
						if parts[1] != "-" {
							if c, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
								stats.Deletions += c
							}
						}
						files.Add(parts[2])
					}
				}
			}
			if err = scanner.Err(); err != nil {
				_ = stdoutReader.Close()
				return fmt.Errorf("GetCodeActivityStats scan: %w", err)
			}
			a := make([]*CodeActivityAuthor, 0, len(authors))
			for _, v := range authors {
				a = append(a, v)
			}
			// Sort authors descending depending on commit count
			sort.Slice(a, func(i, j int) bool {
				return a[i].Commits > a[j].Commits
			})
			stats.AuthorCount = int64(len(authors))
			stats.ChangedFiles = int64(len(files))
			stats.Authors = a
			_ = stdoutReader.Close()
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get GetCodeActivityStats for repository.\nError: %w\nStderr: %s", err, stderr)
	}

	return stats, nil
}
