// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import (
	"container/list"
	"strings"
	"time"
)

//
// |   _   _
// |_ (_) (_|
//        _|

// LogService represents a service that provides log information
type LogService interface {
	// GetCommitByPath returns the last commit of relative path.
	GetCommitByPath(repo Repository, relpath string) (Commit, error)

	// GetCommitByPathWithID returns the last commit of relative path from ID.
	GetCommitByPathWithID(repo Repository, id Hash, relpath string) (Commit, error)

	// FileCommitsCount return the number of files at a revison
	FileCommitsCount(repo Repository, revision, file string) (int64, error)

	// GetFilesChanged returns the files changed between two treeishs
	GetFilesChanged(repo Repository, id1, id2 string) ([]string, error)

	// FileChangedBetweenCommits Returns true if the file changed between commit IDs id1 and id2
	// You must ensure that id1 and id2 are valid commit ids.
	FileChangedBetweenCommits(repo Repository, filename, id1, id2 string) (bool, error)

	// CommitsByFileAndRange return the commits according revison file and the page
	CommitsByFileAndRange(repo Repository, revision, file string, page, pageSize int) (*list.List, error)

	// CommitsByFileAndRangeNoFollow return the commits according revison file and the page
	CommitsByFileAndRangeNoFollow(repo Repository, revision, file string, page, pageSize int) (*list.List, error)

	// CommitsBefore the limit is depth, not total number of returned commits.
	CommitsBefore(repo Repository, revision string, limit int) (*list.List, error)

	// FilesCountBetween return the number of files changed between two commits
	FilesCountBetween(repo Repository, startCommitID, endCommitID string) (int, error)

	// CommitsBetween returns a list that contains commits between [last, before).
	CommitsBetween(repo Repository, last Commit, before Commit) (*list.List, error)

	// CommitsBetweenIDs return commits between two commits
	CommitsBetweenIDs(repo Repository, last, before string) (*list.List, error)

	// GetCommitsFromIDs get commits from commit IDs
	GetCommitsFromIDs(repo Repository, commitIDs []string) (*list.List, error)

	// CommitsBetweenLimit returns a list that contains at most limit commits skipping the first skip commits between [last, before)
	CommitsBetweenLimit(repo Repository, last Commit, before Commit, limit, skip int) (*list.List, error)

	// CommitsCountBetween return numbers of commits between two commits
	CommitsCountBetween(repo Repository, start, end string) (int64, error)

	// GetAllCommitsCount returns count of all commits in repository
	GetAllCommitsCount(repo Repository) (int64, error)

	// GetLatestCommitTime returns time for latest commit in repository (across all branches)
	GetLatestCommitTime(repoPath string) (time.Time, error)

	// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
	GetFullCommitID(repoPath, shortID string) (string, error)

	// GetBranches returns the branches for this commit in the provided repository
	GetBranches(repo Repository, commit Commit, limit int) ([]string, error)

	// SearchCommits searches commits
	SearchCommits(repo Repository, revision string, opts SearchCommitsOptions) (*list.List, error)
}

// SearchCommitsOptions specify the parameters for SearchCommits
type SearchCommitsOptions struct {
	Keywords            []string
	Authors, Committers []string
	After, Before       string
	All                 bool
}

// NewSearchCommitsOptions construct a SearchCommitsOption from a space-delimited search string
func NewSearchCommitsOptions(searchString string, forAllRefs bool) SearchCommitsOptions {
	var keywords, authors, committers []string
	var after, before string

	fields := strings.Fields(searchString)
	for _, k := range fields {
		switch {
		case strings.HasPrefix(k, "author:"):
			authors = append(authors, strings.TrimPrefix(k, "author:"))
		case strings.HasPrefix(k, "committer:"):
			committers = append(committers, strings.TrimPrefix(k, "committer:"))
		case strings.HasPrefix(k, "after:"):
			after = strings.TrimPrefix(k, "after:")
		case strings.HasPrefix(k, "before:"):
			before = strings.TrimPrefix(k, "before:")
		default:
			keywords = append(keywords, k)
		}
	}

	return SearchCommitsOptions{
		Keywords:   keywords,
		Authors:    authors,
		Committers: committers,
		After:      after,
		Before:     before,
		All:        forAllRefs,
	}
}
