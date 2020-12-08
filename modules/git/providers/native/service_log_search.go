// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"container/list"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// SearchCommits searches commits
func (LogService) SearchCommits(repo service.Repository, revision string, opts service.SearchCommitsOptions) (*list.List, error) {
	// create new git log command with limit of 100 commis
	cmd := git.NewCommand("log", revision, "-100", LogHashFormat)
	// ignore case
	args := []string{"-i"}

	// add authors if present in search query
	if len(opts.Authors) > 0 {
		for _, v := range opts.Authors {
			args = append(args, "--author="+v)
		}
	}

	// add commiters if present in search query
	if len(opts.Committers) > 0 {
		for _, v := range opts.Committers {
			args = append(args, "--committer="+v)
		}
	}

	// add time constraints if present in search query
	if len(opts.After) > 0 {
		args = append(args, "--after="+opts.After)
	}
	if len(opts.Before) > 0 {
		args = append(args, "--before="+opts.Before)
	}

	// pretend that all refs along with HEAD were listed on command line as <commis>
	// https://git-scm.com/docs/git-log#Documentation/git-log.txt---all
	// note this is done only for command created above
	if opts.All {
		cmd.AddArguments("--all")
	}

	// add remaining keywords from search string
	// note this is done only for command created above
	if len(opts.Keywords) > 0 {
		for _, v := range opts.Keywords {
			cmd.AddArguments("--grep=" + v)
		}
	}

	// search for commits matching given constraints and keywords in commit msg
	cmd.AddArguments(args...)
	stdout, err := cmd.RunInDirBytes(repo.Path())
	if err != nil {
		return nil, err
	}
	if len(stdout) != 0 {
		stdout = append(stdout, '\n')
	}

	// if there are any keywords (ie not commiter:, author:, time:)
	// then let's iterate over them
	if len(opts.Keywords) > 0 {
		for _, v := range opts.Keywords {
			// ignore anything below 4 characters as too unspecific
			if len(v) >= 4 {
				// create new git log command with 1 commit limit
				hashCmd := git.NewCommand("log", "-1", LogHashFormat)
				// add previous arguments except for --grep and --all
				hashCmd.AddArguments(args...)
				// add keyword as <commit>
				hashCmd.AddArguments(v)

				// search with given constraints for commit matching sha hash of v
				hashMatching, err := hashCmd.RunInDirBytes(repo.Path())
				if err != nil || bytes.Contains(stdout, hashMatching) {
					continue
				}
				stdout = append(stdout, hashMatching...)
				stdout = append(stdout, '\n')
			}
		}
	}
	return BatchReadCommits(repo, bytes.NewReader(stdout))
}
