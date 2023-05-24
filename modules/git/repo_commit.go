// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(name string) (string, error) {
	return repo.GetRefCommitID(BranchPrefix + name)
}

// GetTagCommitID returns last commit ID string of given tag.
func (repo *Repository) GetTagCommitID(name string) (string, error) {
	return repo.GetRefCommitID(TagPrefix + name)
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(commitID string) (*Commit, error) {
	id, err := repo.ConvertToSHA1(commitID)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

// GetBranchCommit returns the last commit of given branch.
func (repo *Repository) GetBranchCommit(name string) (*Commit, error) {
	commitID, err := repo.GetBranchCommitID(name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(commitID)
}

// GetTagCommit get the commit of the specific tag via name
func (repo *Repository) GetTagCommit(name string) (*Commit, error) {
	commitID, err := repo.GetTagCommitID(name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(commitID)
}

func (repo *Repository) getCommitByPathWithID(id SHA1, relpath string) (*Commit, error) {
	// File name starts with ':' must be escaped.
	if relpath[0] == ':' {
		relpath = `\` + relpath
	}

	stdout, _, runErr := NewCommand(repo.Ctx, "log", "-1", prettyLogFormat).AddDynamicArguments(id.String()).AddDashesAndList(relpath).RunStdString(&RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}

	id, err := NewIDFromString(stdout)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

// GetCommitByPath returns the last commit of relative path.
func (repo *Repository) GetCommitByPath(relpath string) (*Commit, error) {
	stdout, _, runErr := NewCommand(repo.Ctx, "log", "-1", prettyLogFormat).AddDashesAndList(relpath).RunStdBytes(&RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}

	commits, err := repo.parsePrettyFormatLogToList(stdout)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, ErrNotExist{ID: relpath}
	}
	return commits[0], nil
}

func (repo *Repository) commitsByRange(id SHA1, page, pageSize int, not string) ([]*Commit, error) {
	cmd := NewCommand(repo.Ctx, "log").
		AddOptionFormat("--skip=%d", (page-1)*pageSize).
		AddOptionFormat("--max-count=%d", pageSize).
		AddArguments(prettyLogFormat).
		AddDynamicArguments(id.String())

	if not != "" {
		cmd.AddOptionValues("--not", not)
	}

	stdout, _, err := cmd.RunStdBytes(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}

	return repo.parsePrettyFormatLogToList(stdout)
}

func (repo *Repository) searchCommits(id SHA1, opts SearchCommitsOptions) ([]*Commit, error) {
	// add common arguments to git command
	addCommonSearchArgs := func(c *Command) {
		// ignore case
		c.AddArguments("-i")

		// add authors if present in search query
		if len(opts.Authors) > 0 {
			for _, v := range opts.Authors {
				c.AddOptionFormat("--author=%s", v)
			}
		}

		// add committers if present in search query
		if len(opts.Committers) > 0 {
			for _, v := range opts.Committers {
				c.AddOptionFormat("--committer=%s", v)
			}
		}

		// add time constraints if present in search query
		if len(opts.After) > 0 {
			c.AddOptionFormat("--after=%s", opts.After)
		}
		if len(opts.Before) > 0 {
			c.AddOptionFormat("--before=%s", opts.Before)
		}
	}

	// create new git log command with limit of 100 commits
	cmd := NewCommand(repo.Ctx, "log", "-100", prettyLogFormat).AddDynamicArguments(id.String())

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
			cmd.AddOptionFormat("--grep=%s", v)
		}
	}

	// search for commits matching given constraints and keywords in commit msg
	addCommonSearchArgs(cmd)
	stdout, _, err := cmd.RunStdBytes(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}
	if len(stdout) != 0 {
		stdout = append(stdout, '\n')
	}

	// if there are any keywords (ie not committer:, author:, time:)
	// then let's iterate over them
	if len(opts.Keywords) > 0 {
		for _, v := range opts.Keywords {
			// ignore anything not matching a valid sha pattern
			if IsValidSHAPattern(v) {
				// create new git log command with 1 commit limit
				hashCmd := NewCommand(repo.Ctx, "log", "-1", prettyLogFormat)
				// add previous arguments except for --grep and --all
				addCommonSearchArgs(hashCmd)
				// add keyword as <commit>
				hashCmd.AddDynamicArguments(v)

				// search with given constraints for commit matching sha hash of v
				hashMatching, _, err := hashCmd.RunStdBytes(&RunOpts{Dir: repo.Path})
				if err != nil || bytes.Contains(stdout, hashMatching) {
					continue
				}
				stdout = append(stdout, hashMatching...)
				stdout = append(stdout, '\n')
			}
		}
	}

	return repo.parsePrettyFormatLogToList(bytes.TrimSuffix(stdout, []byte{'\n'}))
}

// FileChangedBetweenCommits Returns true if the file changed between commit IDs id1 and id2
// You must ensure that id1 and id2 are valid commit ids.
func (repo *Repository) FileChangedBetweenCommits(filename, id1, id2 string) (bool, error) {
	stdout, _, err := NewCommand(repo.Ctx, "diff", "--name-only", "-z").AddDynamicArguments(id1, id2).AddDashesAndList(filename).RunStdBytes(&RunOpts{Dir: repo.Path})
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(stdout))) > 0, nil
}

// FileCommitsCount return the number of files at a revision
func (repo *Repository) FileCommitsCount(revision, file string) (int64, error) {
	return CommitsCount(repo.Ctx,
		CommitsCountOptions{
			RepoPath: repo.Path,
			Revision: []string{revision},
			RelPath:  []string{file},
		})
}

type CommitsByFileAndRangeOptions struct {
	Revision string
	File     string
	Not      string
	Page     int
}

// CommitsByFileAndRange return the commits according revision file and the page
func (repo *Repository) CommitsByFileAndRange(opts CommitsByFileAndRangeOptions) ([]*Commit, error) {
	skip := (opts.Page - 1) * setting.Git.CommitsRangeSize

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	go func() {
		stderr := strings.Builder{}
		gitCmd := NewCommand(repo.Ctx, "rev-list").
			AddOptionFormat("--max-count=%d", setting.Git.CommitsRangeSize*opts.Page).
			AddOptionFormat("--skip=%d", skip)
		gitCmd.AddDynamicArguments(opts.Revision)

		if opts.Not != "" {
			gitCmd.AddOptionValues("--not", opts.Not)
		}

		gitCmd.AddDashesAndList(opts.File)
		err := gitCmd.Run(&RunOpts{
			Dir:    repo.Path,
			Stdout: stdoutWriter,
			Stderr: &stderr,
		})
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	commits := []*Commit{}
	shaline := [41]byte{}
	var sha1 SHA1
	for {
		n, err := io.ReadFull(stdoutReader, shaline[:])
		if err != nil || n < 40 {
			if err == io.EOF {
				err = nil
			}
			return commits, err
		}
		n, err = hex.Decode(sha1[:], shaline[0:40])
		if n != 20 {
			err = fmt.Errorf("invalid sha %q", string(shaline[:40]))
		}
		if err != nil {
			return nil, err
		}
		commit, err := repo.getCommit(sha1)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
}

// FilesCountBetween return the number of files changed between two commits
func (repo *Repository) FilesCountBetween(startCommitID, endCommitID string) (int, error) {
	stdout, _, err := NewCommand(repo.Ctx, "diff", "--name-only").AddDynamicArguments(startCommitID + "..." + endCommitID).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// git >= 2.28 now returns an error if startCommitID and endCommitID have become unrelated.
		// previously it would return the results of git diff --name-only startCommitID endCommitID so let's try that...
		stdout, _, err = NewCommand(repo.Ctx, "diff", "--name-only").AddDynamicArguments(startCommitID, endCommitID).RunStdString(&RunOpts{Dir: repo.Path})
	}
	if err != nil {
		return 0, err
	}
	return len(strings.Split(stdout, "\n")) - 1, nil
}

// CommitsBetween returns a list that contains commits between [before, last).
// If before is detached (removed by reset + push) it is not included.
func (repo *Repository) CommitsBetween(last, before *Commit) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
	} else {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(before.ID.String() + ".." + last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list before last so let's try that...
			stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(before.ID.String(), last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
}

// CommitsBetweenLimit returns a list that contains at most limit commits skipping the first skip commits between [before, last)
func (repo *Repository) CommitsBetweenLimit(last, before *Commit, limit, skip int) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").
			AddOptionValues("--max-count", strconv.Itoa(limit)).
			AddOptionValues("--skip", strconv.Itoa(skip)).
			AddDynamicArguments(last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
	} else {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").
			AddOptionValues("--max-count", strconv.Itoa(limit)).
			AddOptionValues("--skip", strconv.Itoa(skip)).
			AddDynamicArguments(before.ID.String() + ".." + last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list --max-count n before last so let's try that...
			stdout, _, err = NewCommand(repo.Ctx, "rev-list").
				AddOptionValues("--max-count", strconv.Itoa(limit)).
				AddOptionValues("--skip", strconv.Itoa(skip)).
				AddDynamicArguments(before.ID.String(), last.ID.String()).RunStdBytes(&RunOpts{Dir: repo.Path})
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
}

// CommitsBetweenNotBase returns a list that contains commits between [before, last), excluding commits in baseBranch.
// If before is detached (removed by reset + push) it is not included.
func (repo *Repository) CommitsBetweenNotBase(last, before *Commit, baseBranch string) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(last.ID.String()).AddOptionValues("--not", baseBranch).RunStdBytes(&RunOpts{Dir: repo.Path})
	} else {
		stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(before.ID.String()+".."+last.ID.String()).AddOptionValues("--not", baseBranch).RunStdBytes(&RunOpts{Dir: repo.Path})
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list before last so let's try that...
			stdout, _, err = NewCommand(repo.Ctx, "rev-list").AddDynamicArguments(before.ID.String(), last.ID.String()).AddOptionValues("--not", baseBranch).RunStdBytes(&RunOpts{Dir: repo.Path})
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
}

// CommitsBetweenIDs return commits between twoe commits
func (repo *Repository) CommitsBetweenIDs(last, before string) ([]*Commit, error) {
	lastCommit, err := repo.GetCommit(last)
	if err != nil {
		return nil, err
	}
	if before == "" {
		return repo.CommitsBetween(lastCommit, nil)
	}
	beforeCommit, err := repo.GetCommit(before)
	if err != nil {
		return nil, err
	}
	return repo.CommitsBetween(lastCommit, beforeCommit)
}

// CommitsCountBetween return numbers of commits between two commits
func (repo *Repository) CommitsCountBetween(start, end string) (int64, error) {
	count, err := CommitsCount(repo.Ctx, CommitsCountOptions{
		RepoPath: repo.Path,
		Revision: []string{start + ".." + end},
	})

	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		return CommitsCount(repo.Ctx, CommitsCountOptions{
			RepoPath: repo.Path,
			Revision: []string{start, end},
		})
	}

	return count, err
}

// commitsBefore the limit is depth, not total number of returned commits.
func (repo *Repository) commitsBefore(id SHA1, limit int) ([]*Commit, error) {
	cmd := NewCommand(repo.Ctx, "log", prettyLogFormat)
	if limit > 0 {
		cmd.AddOptionFormat("-%d", limit)
	}
	cmd.AddDynamicArguments(id.String())

	stdout, _, runErr := cmd.RunStdBytes(&RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}

	formattedLog, err := repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}

	commits := make([]*Commit, 0, len(formattedLog))
	for _, commit := range formattedLog {
		branches, err := repo.getBranches(commit, 2)
		if err != nil {
			return nil, err
		}

		if len(branches) > 1 {
			break
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

func (repo *Repository) getCommitsBefore(id SHA1) ([]*Commit, error) {
	return repo.commitsBefore(id, 0)
}

func (repo *Repository) getCommitsBeforeLimit(id SHA1, num int) ([]*Commit, error) {
	return repo.commitsBefore(id, num)
}

func (repo *Repository) getBranches(commit *Commit, limit int) ([]string, error) {
	if CheckGitVersionAtLeast("2.7.0") == nil {
		stdout, _, err := NewCommand(repo.Ctx, "for-each-ref", "--format=%(refname:strip=2)").
			AddOptionFormat("--count=%d", limit).
			AddOptionValues("--contains", commit.ID.String(), BranchPrefix).
			RunStdString(&RunOpts{Dir: repo.Path})
		if err != nil {
			return nil, err
		}

		branches := strings.Fields(stdout)
		return branches, nil
	}

	stdout, _, err := NewCommand(repo.Ctx, "branch").AddOptionValues("--contains", commit.ID.String()).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}

	refs := strings.Split(stdout, "\n")

	var max int
	if len(refs) > limit {
		max = limit
	} else {
		max = len(refs) - 1
	}

	branches := make([]string, max)
	for i, ref := range refs[:max] {
		parts := strings.Fields(ref)

		branches[i] = parts[len(parts)-1]
	}
	return branches, nil
}

// GetCommitsFromIDs get commits from commit IDs
func (repo *Repository) GetCommitsFromIDs(commitIDs []string) []*Commit {
	commits := make([]*Commit, 0, len(commitIDs))

	for _, commitID := range commitIDs {
		commit, err := repo.GetCommit(commitID)
		if err == nil && commit != nil {
			commits = append(commits, commit)
		}
	}

	return commits
}

// IsCommitInBranch check if the commit is on the branch
func (repo *Repository) IsCommitInBranch(commitID, branch string) (r bool, err error) {
	stdout, _, err := NewCommand(repo.Ctx, "branch", "--contains").AddDynamicArguments(commitID, branch).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		return false, err
	}
	return len(stdout) > 0, err
}

func (repo *Repository) AddLastCommitCache(cacheKey, fullName, sha string) error {
	if repo.LastCommitCache == nil {
		commitsCount, err := cache.GetInt64(cacheKey, func() (int64, error) {
			commit, err := repo.GetCommit(sha)
			if err != nil {
				return 0, err
			}
			return commit.CommitsCount()
		})
		if err != nil {
			return err
		}
		repo.LastCommitCache = NewLastCommitCache(commitsCount, fullName, repo, cache.GetCache())
	}
	return nil
}
