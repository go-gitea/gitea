// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
)

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(ctx context.Context, name string) (string, error) {
	return repo.GetRefCommitID(ctx, BranchPrefix+name)
}

// GetTagCommitID returns last commit ID string of given tag.
func (repo *Repository) GetTagCommitID(ctx context.Context, name string) (string, error) {
	return repo.GetRefCommitID(ctx, TagPrefix+name)
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(ctx context.Context, commitID string) (*Commit, error) {
	id, err := repo.ConvertToGitID(ctx, commitID)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(ctx, id)
}

// GetBranchCommit returns the last commit of given branch.
func (repo *Repository) GetBranchCommit(ctx context.Context, name string) (*Commit, error) {
	commitID, err := repo.GetBranchCommitID(ctx, name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(ctx, commitID)
}

// GetTagCommit get the commit of the specific tag via name
func (repo *Repository) GetTagCommit(ctx context.Context, name string) (*Commit, error) {
	commitID, err := repo.GetTagCommitID(ctx, name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(ctx, commitID)
}

func (repo *Repository) getCommitByPathWithID(ctx context.Context, id ObjectID, relpath string) (*Commit, error) {
	// File name starts with ':' must be escaped.
	if relpath[0] == ':' {
		relpath = `\` + relpath
	}

	stdout, _, runErr := gitcmd.NewCommand("log", "-1", prettyLogFormat).
		AddDynamicArguments(id.String()).
		AddDashesAndList(relpath).
		WithDir(repo.Path).
		RunStdString(ctx)
	if runErr != nil {
		return nil, runErr
	}

	id, err := NewIDFromString(stdout)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(ctx, id)
}

// GetCommitByPath returns the last commit of relative path.
func (repo *Repository) GetCommitByPath(ctx context.Context, relpath string) (*Commit, error) {
	stdout, _, runErr := gitcmd.NewCommand("log", "-1", prettyLogFormat).
		AddDashesAndList(relpath).
		WithDir(repo.Path).
		RunStdBytes(ctx)
	if runErr != nil {
		return nil, runErr
	}

	commits, err := repo.parsePrettyFormatLogToList(ctx, stdout)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, ErrNotExist{ID: relpath}
	}
	return commits[0], nil
}

// commitsByRangeWithTime returns the specific page commits before current revision, with not, since, until support
func (repo *Repository) commitsByRangeWithTime(ctx context.Context, id ObjectID, page, pageSize int, not, since, until string) ([]*Commit, error) {
	cmd := gitcmd.NewCommand("log").
		AddOptionFormat("--skip=%d", (page-1)*pageSize).
		AddOptionFormat("--max-count=%d", pageSize).
		AddArguments(prettyLogFormat).
		AddDynamicArguments(id.String())

	if not != "" {
		cmd.AddOptionValues("--not", not)
	}
	if since != "" {
		cmd.AddOptionFormat("--since=%s", since)
	}
	if until != "" {
		cmd.AddOptionFormat("--until=%s", until)
	}

	stdout, _, err := cmd.WithDir(repo.Path).RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}

	return repo.parsePrettyFormatLogToList(ctx, stdout)
}

func (repo *Repository) searchCommits(ctx context.Context, id ObjectID, opts SearchCommitsOptions) ([]*Commit, error) {
	// add common arguments to git command
	addCommonSearchArgs := func(c *gitcmd.Command) {
		// ignore case
		c.AddArguments("-i")

		// add authors if present in search query
		for _, v := range opts.Authors {
			c.AddOptionFormat("--author=%s", v)
		}

		// add committers if present in search query
		for _, v := range opts.Committers {
			c.AddOptionFormat("--committer=%s", v)
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
	cmd := gitcmd.NewCommand("log", "-100", prettyLogFormat).AddDynamicArguments(id.String())

	// pretend that all refs along with HEAD were listed on command line as <commis>
	// https://git-scm.com/docs/git-log#Documentation/git-log.txt---all
	// note this is done only for command created above
	if opts.All {
		cmd.AddArguments("--all")
	}

	// interpret search string keywords as string instead of regex
	cmd.AddArguments("--fixed-strings")

	// add remaining keywords from search string
	// note this is done only for command created above
	for _, v := range opts.Keywords {
		cmd.AddOptionFormat("--grep=%s", v)
	}

	// search for commits matching given constraints and keywords in commit msg
	addCommonSearchArgs(cmd)
	stdout, _, err := cmd.WithDir(repo.Path).RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}
	if len(stdout) != 0 {
		stdout = append(stdout, '\n')
	}

	// if there are any keywords (ie not committer:, author:, time:)
	// then let's iterate over them
	for _, v := range opts.Keywords {
		// ignore anything not matching a valid sha pattern
		if id.Type().IsValid(v) {
			// create new git log command with 1 commit limit
			hashCmd := gitcmd.NewCommand("log", "-1", prettyLogFormat)
			// add previous arguments except for --grep and --all
			addCommonSearchArgs(hashCmd)
			// add keyword as <commit>
			hashCmd.AddDynamicArguments(v)

			// search with given constraints for commit matching sha hash of v
			hashMatching, _, err := hashCmd.WithDir(repo.Path).RunStdBytes(ctx)
			if err != nil || bytes.Contains(stdout, hashMatching) {
				continue
			}
			stdout = append(stdout, hashMatching...)
			stdout = append(stdout, '\n')
		}
	}

	return repo.parsePrettyFormatLogToList(ctx, bytes.TrimSuffix(stdout, []byte{'\n'}))
}

// FileChangedBetweenCommits Returns true if the file changed between commit IDs id1 and id2
// You must ensure that id1 and id2 are valid commit ids.
func (repo *Repository) FileChangedBetweenCommits(ctx context.Context, filename, id1, id2 string) (bool, error) {
	stdout, _, err := gitcmd.NewCommand("diff", "--name-only", "-z").
		AddDynamicArguments(id1, id2).
		AddDashesAndList(filename).
		WithDir(repo.Path).
		RunStdBytes(ctx)
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(stdout))) > 0, nil
}

// FileCommitsCount return the number of files at a revision
func (repo *Repository) FileCommitsCount(ctx context.Context, revision, file string) (int64, error) {
	return CommitsCount(ctx,
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
	Since    string
	Until    string
}

// CommitsByFileAndRange return the commits according revision file and the page
func (repo *Repository) CommitsByFileAndRange(ctx context.Context, opts CommitsByFileAndRangeOptions) ([]*Commit, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	go func() {
		stderr := strings.Builder{}
		gitCmd := gitcmd.NewCommand("rev-list").
			AddOptionFormat("--max-count=%d", setting.Git.CommitsRangeSize).
			AddOptionFormat("--skip=%d", (opts.Page-1)*setting.Git.CommitsRangeSize)
		gitCmd.AddDynamicArguments(opts.Revision)

		if opts.Not != "" {
			gitCmd.AddOptionValues("--not", opts.Not)
		}
		if opts.Since != "" {
			gitCmd.AddOptionFormat("--since=%s", opts.Since)
		}
		if opts.Until != "" {
			gitCmd.AddOptionFormat("--until=%s", opts.Until)
		}

		gitCmd.AddDashesAndList(opts.File)
		err := gitCmd.WithDir(repo.Path).
			WithStdout(stdoutWriter).
			WithStderr(&stderr).
			Run(ctx)
		if err != nil {
			_ = stdoutWriter.CloseWithError(gitcmd.ConcatenateError(err, (&stderr).String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return nil, err
	}

	length := objectFormat.FullLength()
	commits := []*Commit{}
	shaline := make([]byte, length+1)
	for {
		n, err := io.ReadFull(stdoutReader, shaline)
		if err != nil || n < length {
			if err == io.EOF {
				err = nil
			}
			return commits, err
		}
		objectID, err := NewIDFromString(string(shaline[0:length]))
		if err != nil {
			return nil, err
		}
		commit, err := repo.getCommit(ctx, objectID)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
}

// FilesCountBetween return the number of files changed between two commits
func (repo *Repository) FilesCountBetween(ctx context.Context, startCommitID, endCommitID string) (int, error) {
	stdout, _, err := gitcmd.NewCommand("diff", "--name-only").
		AddDynamicArguments(startCommitID + "..." + endCommitID).
		WithDir(repo.Path).
		RunStdString(ctx)
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// git >= 2.28 now returns an error if startCommitID and endCommitID have become unrelated.
		// previously it would return the results of git diff --name-only startCommitID endCommitID so let's try that...
		stdout, _, err = gitcmd.NewCommand("diff", "--name-only").
			AddDynamicArguments(startCommitID, endCommitID).
			WithDir(repo.Path).
			RunStdString(ctx)
	}
	if err != nil {
		return 0, err
	}
	return len(strings.Split(stdout, "\n")) - 1, nil
}

// CommitsBetween returns a list that contains commits between [before, last).
// If before is detached (removed by reset + push) it is not included.
func (repo *Repository) CommitsBetween(ctx context.Context, last, before *Commit) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddDynamicArguments(last.ID.String()).
			WithDir(repo.Path).
			RunStdBytes(ctx)
	} else {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddDynamicArguments(before.ID.String() + ".." + last.ID.String()).
			WithDir(repo.Path).
			RunStdBytes(ctx)
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list before last so let's try that...
			stdout, _, err = gitcmd.NewCommand("rev-list").
				AddDynamicArguments(before.ID.String(), last.ID.String()).
				WithDir(repo.Path).
				RunStdBytes(ctx)
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(ctx, bytes.TrimSpace(stdout))
}

// CommitsBetweenLimit returns a list that contains at most limit commits skipping the first skip commits between [before, last)
func (repo *Repository) CommitsBetweenLimit(ctx context.Context, last, before *Commit, limit, skip int) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddOptionValues("--max-count", strconv.Itoa(limit)).
			AddOptionValues("--skip", strconv.Itoa(skip)).
			AddDynamicArguments(last.ID.String()).
			WithDir(repo.Path).
			RunStdBytes(ctx)
	} else {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddOptionValues("--max-count", strconv.Itoa(limit)).
			AddOptionValues("--skip", strconv.Itoa(skip)).
			AddDynamicArguments(before.ID.String() + ".." + last.ID.String()).
			WithDir(repo.Path).
			RunStdBytes(ctx)
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list --max-count n before last so let's try that...
			stdout, _, err = gitcmd.NewCommand("rev-list").
				AddOptionValues("--max-count", strconv.Itoa(limit)).
				AddOptionValues("--skip", strconv.Itoa(skip)).
				AddDynamicArguments(before.ID.String(), last.ID.String()).
				WithDir(repo.Path).
				RunStdBytes(ctx)
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(ctx, bytes.TrimSpace(stdout))
}

// CommitsBetweenNotBase returns a list that contains commits between [before, last), excluding commits in baseBranch.
// If before is detached (removed by reset + push) it is not included.
func (repo *Repository) CommitsBetweenNotBase(ctx context.Context, last, before *Commit, baseBranch string) ([]*Commit, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddDynamicArguments(last.ID.String()).
			AddOptionValues("--not", baseBranch).
			WithDir(repo.Path).
			RunStdBytes(ctx)
	} else {
		stdout, _, err = gitcmd.NewCommand("rev-list").
			AddDynamicArguments(before.ID.String()+".."+last.ID.String()).
			AddOptionValues("--not", baseBranch).
			WithDir(repo.Path).
			RunStdBytes(ctx)
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// previously it would return the results of git rev-list before last so let's try that...
			stdout, _, err = gitcmd.NewCommand("rev-list").
				AddDynamicArguments(before.ID.String(), last.ID.String()).
				AddOptionValues("--not", baseBranch).
				WithDir(repo.Path).
				RunStdBytes(ctx)
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(ctx, bytes.TrimSpace(stdout))
}

// CommitsBetweenIDs return commits between twoe commits
func (repo *Repository) CommitsBetweenIDs(ctx context.Context, last, before string) ([]*Commit, error) {
	lastCommit, err := repo.GetCommit(ctx, last)
	if err != nil {
		return nil, err
	}
	if before == "" {
		return repo.CommitsBetween(ctx, lastCommit, nil)
	}
	beforeCommit, err := repo.GetCommit(ctx, before)
	if err != nil {
		return nil, err
	}
	return repo.CommitsBetween(ctx, lastCommit, beforeCommit)
}

// CommitsCountBetween return numbers of commits between two commits
func (repo *Repository) CommitsCountBetween(ctx context.Context, start, end string) (int64, error) {
	count, err := CommitsCount(ctx, CommitsCountOptions{
		RepoPath: repo.Path,
		Revision: []string{start + ".." + end},
	})

	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		return CommitsCount(ctx, CommitsCountOptions{
			RepoPath: repo.Path,
			Revision: []string{start, end},
		})
	}

	return count, err
}

// commitsBefore the limit is depth, not total number of returned commits.
func (repo *Repository) commitsBefore(ctx context.Context, id ObjectID, limit int) ([]*Commit, error) {
	cmd := gitcmd.NewCommand("log", prettyLogFormat)
	if limit > 0 {
		cmd.AddOptionFormat("-%d", limit)
	}
	cmd.AddDynamicArguments(id.String())

	stdout, _, runErr := cmd.WithDir(repo.Path).RunStdBytes(ctx)
	if runErr != nil {
		return nil, runErr
	}

	formattedLog, err := repo.parsePrettyFormatLogToList(ctx, bytes.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}

	commits := make([]*Commit, 0, len(formattedLog))
	for _, commit := range formattedLog {
		branches, err := repo.getBranches(ctx, os.Environ(), commit.ID.String(), 2)
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

func (repo *Repository) getCommitsBefore(ctx context.Context, id ObjectID) ([]*Commit, error) {
	return repo.commitsBefore(ctx, id, 0)
}

func (repo *Repository) getCommitsBeforeLimit(ctx context.Context, id ObjectID, num int) ([]*Commit, error) {
	return repo.commitsBefore(ctx, id, num)
}

func (repo *Repository) getBranches(ctx context.Context, env []string, commitID string, limit int) ([]string, error) {
	if DefaultFeatures().CheckVersionAtLeast("2.7.0") {
		stdout, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname:strip=2)").
			AddOptionFormat("--count=%d", limit).
			AddOptionValues("--contains", commitID, BranchPrefix).
			WithDir(repo.Path).
			WithEnv(env).
			RunStdString(ctx)
		if err != nil {
			return nil, err
		}

		branches := strings.Fields(stdout)
		return branches, nil
	}

	stdout, _, err := gitcmd.NewCommand("branch").
		AddOptionValues("--contains", commitID).
		WithDir(repo.Path).
		WithEnv(env).
		RunStdString(ctx)
	if err != nil {
		return nil, err
	}

	refs := strings.Split(stdout, "\n")

	var maxNum int
	if len(refs) > limit {
		maxNum = limit
	} else {
		maxNum = len(refs) - 1
	}

	branches := make([]string, maxNum)
	for i, ref := range refs[:maxNum] {
		parts := strings.Fields(ref)

		branches[i] = parts[len(parts)-1]
	}
	return branches, nil
}

// GetCommitsFromIDs get commits from commit IDs
func (repo *Repository) GetCommitsFromIDs(ctx context.Context, commitIDs []string) []*Commit {
	commits := make([]*Commit, 0, len(commitIDs))

	for _, commitID := range commitIDs {
		commit, err := repo.GetCommit(ctx, commitID)
		if err == nil && commit != nil {
			commits = append(commits, commit)
		}
	}

	return commits
}

// IsCommitInBranch check if the commit is on the branch
func (repo *Repository) IsCommitInBranch(ctx context.Context, commitID, branch string) (r bool, err error) {
	stdout, _, err := gitcmd.NewCommand("branch", "--contains").
		AddDynamicArguments(commitID, branch).
		WithDir(repo.Path).
		RunStdString(ctx)
	if err != nil {
		return false, err
	}
	return len(stdout) > 0, err
}

func (repo *Repository) AddLastCommitCache(ctx context.Context, cacheKey, fullName, sha string) error {
	if repo.LastCommitCache == nil {
		commitsCount, err := cache.GetInt64(ctx, cacheKey, func(ctx context.Context) (int64, error) {
			commit, err := repo.GetCommit(ctx, sha)
			if err != nil {
				return 0, err
			}
			return commit.CommitsCount(ctx)
		})
		if err != nil {
			return err
		}
		repo.LastCommitCache = NewLastCommitCache(commitsCount, fullName, repo, cache.GetCache())
	}
	return nil
}

// GetCommitBranchStart returns the commit where the branch diverged
func (repo *Repository) GetCommitBranchStart(ctx context.Context, env []string, branch, endCommitID string) (string, error) {
	cmd := gitcmd.NewCommand("log", prettyLogFormat)
	cmd.AddDynamicArguments(endCommitID)

	stdout, _, runErr := cmd.WithDir(repo.Path).
		WithEnv(env).
		RunStdBytes(ctx)
	if runErr != nil {
		return "", runErr
	}

	parts := bytes.SplitSeq(bytes.TrimSpace(stdout), []byte{'\n'})

	// check the commits one by one until we find a commit contained by another branch
	// and we think this commit is the divergence point
	for commitID := range parts {
		branches, err := repo.getBranches(ctx, env, string(commitID), 2)
		if err != nil {
			return "", err
		}
		for _, b := range branches {
			if b != branch {
				return string(commitID), nil
			}
		}
	}

	return "", nil
}
