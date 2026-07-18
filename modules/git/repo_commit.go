// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
)

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(ctx context.Context, name string) (string, error) {
	return repo.GetRefCommitID(ctx, BranchPrefix+name)
}

// GetTagCommitID returns last commit ID string of given tag.
func (repo *Repository) GetTagCommitID(ctx context.Context, name string) (string, error) {
	return repo.GetRefCommitID(ctx, TagPrefix+name)
}

// GetCommit returns a commit object of by the git ref.
func (repo *Repository) GetCommit(ctx context.Context, ref string) (*Commit, error) {
	id, err := repo.ConvertToGitID(ctx, ref)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(ctx, id)
}

// GetBranchCommit returns the last commit of given branch.
func (repo *Repository) GetBranchCommit(ctx context.Context, name string) (*Commit, error) {
	return repo.GetCommit(ctx, RefNameFromBranch(name).String())
}

// GetTagCommit get the commit of the specific tag via name
func (repo *Repository) GetTagCommit(ctx context.Context, name string) (*Commit, error) {
	return repo.GetCommit(ctx, RefNameFromTag(name).String())
}

const prettyLogFormat = `--pretty=format:%H`

func (repo *Repository) ShowPrettyFormatLogToList(ctx context.Context, revisionRange string) ([]*Commit, error) {
	// avoid: ambiguous argument 'refs/a...refs/b': unknown revision or path not in the working tree. Use '--': 'git <command> [<revision>...] -- [<file>...]'
	logs, _, err := gitcmd.NewCommand("log").AddArguments(prettyLogFormat).
		AddDynamicArguments(revisionRange).AddArguments("--").WithDir(repo.Path).
		RunStdBytes(ctx)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(ctx, logs)
}

func (repo *Repository) parsePrettyFormatLogToList(ctx context.Context, logs []byte) ([]*Commit, error) {
	var commits []*Commit
	if len(logs) == 0 {
		return commits, nil
	}

	parts := bytes.SplitSeq(logs, []byte{'\n'})

	for commitID := range parts {
		commit, err := repo.GetCommit(ctx, string(commitID))
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

func (repo *Repository) getCommitByPathWithID(ctx context.Context, id ObjectID, relpath string) (*Commit, error) {
	// File name starts with ':' must be escaped.
	if strings.HasPrefix(relpath, ":") {
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

type CommitsByFileAndRangeOptions struct {
	Revision string
	File     string
	Not      string
	Page     int
	Since    string
	Until    string

	// when using FollowRename, there is no quick way to know the total count, so use hasMore to indicate if there are more commits to load
	FollowRename bool
}

// CommitsByFileAndRange return the commits according revision file and the page
func (repo *Repository) CommitsByFileAndRange(ctx context.Context, opts CommitsByFileAndRangeOptions) (commits []*Commit, hasMore bool, _ error) {
	limit := setting.Git.CommitsRangeSize
	gitCmd := gitcmd.NewCommand("--no-pager", "log").
		AddArguments("--pretty=tformat:%H").
		AddOptionFormat("--max-count=%d", limit+1).
		AddOptionFormat("--skip=%d", (opts.Page-1)*setting.Git.CommitsRangeSize)
	if opts.FollowRename {
		gitCmd.AddArguments("--follow")
	}
	if opts.Since != "" {
		gitCmd.AddOptionFormat("--since=%s", opts.Since)
	}
	if opts.Until != "" {
		gitCmd.AddOptionFormat("--until=%s", opts.Until)
	}
	gitCmd.AddDynamicArguments(opts.Revision)
	if opts.Not != "" {
		gitCmd.AddOptionValues("--not", opts.Not)
	}
	gitCmd.AddDashesAndList(opts.File)

	stdoutReader, stdoutReaderClose := gitCmd.MakeStdoutPipe()
	defer stdoutReaderClose()
	err := gitCmd.WithDir(repo.Path).
		WithPipelineFunc(func(context gitcmd.Context) error {
			objectFormat, err := repo.GetObjectFormat(ctx)
			if err != nil {
				return err
			}

			length := objectFormat.FullLength()
			shaline := make([]byte, length+1)
			for {
				n, err := io.ReadFull(stdoutReader, shaline)
				if err != nil || n < length {
					if err == io.EOF {
						err = nil
					}
					return err
				}
				objectID, err := NewIDFromString(string(shaline[0:length]))
				if err != nil {
					return err
				}
				commit, err := repo.getCommit(ctx, objectID)
				if err != nil {
					return err
				}
				commits = append(commits, commit)
			}
		}).
		RunWithStderr(ctx)

	hasMore = len(commits) > limit
	if hasMore {
		commits = commits[:limit]
	}
	return commits, hasMore, err
}

// CommitsBetween returns a list that contains commits between [after, before). After is the first item in the slice.
// If "before" and "after" are not related, it returns the all commits for the "after" commit.
func (repo *Repository) CommitsBetween(ctx context.Context, afterRef, beforeRef RefName, limit int, optSkip ...int) ([]*Commit, error) {
	gitCmd := func() *gitcmd.Command {
		cmd := gitcmd.NewCommand("rev-list").WithDir(repo.Path)
		if limit >= 0 {
			cmd.AddOptionValues("--max-count", strconv.Itoa(limit))
		}
		if len(optSkip) > 0 {
			cmd.AddOptionValues("--skip", strconv.Itoa(optSkip[0]))
		}
		return cmd
	}
	var stdout []byte
	var err error
	if beforeRef == "" {
		stdout, _, err = gitCmd().AddDynamicArguments(afterRef.String()).RunStdBytes(ctx)
	} else {
		stdout, _, err = gitCmd().AddDynamicArguments(beforeRef.String() + ".." + afterRef.String()).RunStdBytes(ctx)
		if gitcmd.IsStderr(err, gitcmd.StderrNoMergeBase) {
			// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
			// if the beforeRef and afterRef are not related (no merge base), just get all commits pushed by afterRef
			stdout, _, err = gitCmd().AddDynamicArguments(afterRef.String()).RunStdBytes(ctx)
		}
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(ctx, bytes.TrimSpace(stdout))
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
		branches, err := repo.getBranches(ctx, nil, commit.ID.String(), 2)
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
	stdout, _, err := gitcmd.NewCommand("for-each-ref", "--format=%(refname:strip=2)").
		AddOptionFormat("--count=%d", limit).
		AddOptionValues("--contains", commitID).
		AddArguments(BranchPrefix).
		WithEnv(env).
		WithDir(repo.Path).
		RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	return strings.Fields(stdout), nil
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

	// check the commits one by one until we find a commit contained by another branch,
	// and we think this commit is the divergence point
	for part := range parts {
		commitID := string(part)
		branches, err := repo.getBranches(ctx, env, commitID, 2)
		if err != nil {
			return "", err
		}
		for _, b := range branches {
			if b != branch {
				return commitID, nil
			}
		}
	}

	return "", nil
}
