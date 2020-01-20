// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
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

// GetCodeActivityStats returns code statistics for acitivity page
func (repo *Repository) GetCodeActivityStats(fromTime time.Time, branch string) (*CodeActivityStats, error) {
	stats := &CodeActivityStats{}

	since := fromTime.Format(time.RFC3339)

	stdout, err := NewCommand("rev-list", "--count", "--no-merges", "--branches=*", "--date=iso", fmt.Sprintf("--since='%s'", since)).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	c, err := strconv.ParseInt(strings.TrimSpace(string(stdout)), 10, 64)
	if err != nil {
		return nil, err
	}
	stats.CommitCountInAllBranches = c

	args := []string{"log", "--numstat", "--no-merges", "--pretty=format:---%n%h%n%an%n%ae%n", "--date=iso", fmt.Sprintf("--since='%s'", since)}
	if len(branch) == 0 {
		args = append(args, "--branches=*")
	} else {
		args = append(args, "--first-parent", branch)
	}

	stdout, err = NewCommand(args...).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Split(bufio.ScanLines)
	stats.CommitCount = 0
	stats.Additions = 0
	stats.Deletions = 0
	authors := make(map[string]*CodeActivityAuthor)
	files := make(map[string]bool)
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
				authors[email] = &CodeActivityAuthor{
					Name:    author,
					Email:   email,
					Commits: 0,
				}
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
				if _, ok := files[parts[2]]; !ok {
					files[parts[2]] = true
				}
			}
		}
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

	return stats, nil
}
