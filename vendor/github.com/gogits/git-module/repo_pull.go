// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"container/list"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PullRequestInfo represents needed information for a pull request.
type PullRequestInfo struct {
	MergeBase string
	Commits   *list.List
	NumFiles  int
}

// GetMergeBase checks and returns merge base of two branches.
func (repo *Repository) GetMergeBase(base, head string) (string, error) {
	stdout, err := NewCommand("merge-base", base, head).RunInDir(repo.Path)
	return strings.TrimSpace(stdout), err
}

// GetPullRequestInfo generates and returns pull request information
// between base and head branches of repositories.
func (repo *Repository) GetPullRequestInfo(basePath, baseBranch, headBranch string) (_ *PullRequestInfo, err error) {
	var remoteBranch string

	// We don't need a temporary remote for same repository.
	if repo.Path != basePath {
		// Add a temporary remote
		tmpRemote := strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = repo.AddRemote(tmpRemote, basePath, true); err != nil {
			return nil, fmt.Errorf("AddRemote: %v", err)
		}
		defer repo.RemoveRemote(tmpRemote)

		remoteBranch = "remotes/" + tmpRemote + "/" + baseBranch
	} else {
		remoteBranch = baseBranch
	}

	prInfo := new(PullRequestInfo)
	prInfo.MergeBase, err = repo.GetMergeBase(remoteBranch, headBranch)
	if err != nil {
		return nil, fmt.Errorf("GetMergeBase: %v", err)
	}

	logs, err := NewCommand("log", prInfo.MergeBase+"..."+headBranch, _PRETTY_LOG_FORMAT).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	prInfo.Commits, err = repo.parsePrettyFormatLogToList(logs)
	if err != nil {
		return nil, fmt.Errorf("parsePrettyFormatLogToList: %v", err)
	}

	// Count number of changed files.
	stdout, err := NewCommand("diff", "--name-only", remoteBranch+"..."+headBranch).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	prInfo.NumFiles = len(strings.Split(stdout, "\n")) - 1

	return prInfo, nil
}

// GetPatch generates and returns patch data between given revisions.
func (repo *Repository) GetPatch(base, head string) ([]byte, error) {
	return NewCommand("diff", "-p", "--binary", base, head).RunInDirBytes(repo.Path)
}
