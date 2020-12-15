// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"container/list"
	"io"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.LogService) = LogService{}

// LogService represents a basic implementation of LogService
type LogService struct{}

// GetCommitByPath returns the last commit of relative path.
func (l LogService) GetCommitByPath(repo service.Repository, relpath string) (service.Commit, error) {
	return l.GetCommitByPathWithID(repo, StringHash(""), relpath)
}

// GetCommitByPathWithID returns the last commit of relative path from ID.
func (LogService) GetCommitByPathWithID(repo service.Repository, id service.Hash, relpath string) (service.Commit, error) {
	// File name starts with ':' must be escaped.
	if relpath[0] == ':' {
		relpath = `\` + relpath
	}

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	if id.IsZero() {
		go common.PipeCommand(
			git.NewCommand("log", "-1", "--pretty=raw", "--", relpath),
			repo.Path(),
			stdoutWriter,
			nil)
	} else {
		go common.PipeCommand(
			git.NewCommand("log", "-1", "--pretty=raw", id.String(), "--", relpath),
			repo.Path(),
			stdoutWriter,
			nil)
	}

	bufReader := bufio.NewReader(stdoutReader)
	_, _ = bufReader.Discard(7)
	idStr, err := bufReader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	idStr = idStr[:len(idStr)-1]

	return CommitFromReader(repo, StringHash(idStr), bufReader)
}

// FileCommitsCount return the number of files at a revison
func (LogService) FileCommitsCount(repo service.Repository, revision, file string) (int64, error) {
	return git.CommitsCountFiles(repo.Path(), []string{revision}, []string{file})
}

// GetFilesChanged returns the files changed between two treeishs
func (LogService) GetFilesChanged(repo service.Repository, id1, id2 string) ([]string, error) {
	stdout, err := git.NewCommand("diff", "--name-only", id1, id2).RunInDirBytes(repo.Path())
	if err != nil {
		return nil, err
	}
	return strings.Split(string(stdout), "\n"), nil
}

// FileChangedBetweenCommits Returns true if the file changed between commit IDs id1 and id2
// You must ensure that id1 and id2 are valid commit ids.
func (LogService) FileChangedBetweenCommits(repo service.Repository, filename, id1, id2 string) (bool, error) {
	stdout, err := git.NewCommand("diff", "--name-only", "-z", id1, id2, "--", filename).RunInDirBytes(repo.Path())
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(stdout))) > 0, nil
}

// CommitsByFileAndRange return the commits according revison file and the page
func (LogService) CommitsByFileAndRange(repo service.Repository, revision, file string, page, pageSize int) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(
		git.NewCommand("log", revision, "--follow", "--skip="+strconv.Itoa((page-1)*50),
			"--max-count="+strconv.Itoa(git.CommitsRangeSize), LogHashFormat, "--", file),
		repo.Path(),
		stdoutWriter,
		nil)

	return BatchReadCommits(repo, stdoutReader)
}

// CommitsByFileAndRangeNoFollow return the commits according revison file and the page
func (LogService) CommitsByFileAndRangeNoFollow(repo service.Repository, revision, file string, page, pageSize int) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(
		git.NewCommand("log", revision, "--skip="+strconv.Itoa((page-1)*50),
			"--max-count="+strconv.Itoa(git.CommitsRangeSize), LogHashFormat, "--", file),
		repo.Path(),
		stdoutWriter,
		nil)

	return BatchReadCommits(repo, stdoutReader)
}

// CommitsBefore the limit is depth, not total number of returned commits.
func (l LogService) CommitsBefore(repo service.Repository, revision string, limit int) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	cmd := git.NewCommand("log")
	if limit > 0 {
		cmd.AddArguments("-"+strconv.Itoa(limit), LogHashFormat, revision)
	} else {
		cmd.AddArguments(LogHashFormat, revision)
	}

	go common.PipeCommand(
		cmd,
		repo.Path(),
		stdoutWriter,
		nil)

	formattedLog, err := BatchReadCommits(repo, stdoutReader)
	if err != nil {
		return nil, err
	}

	commits := list.New()
	for logEntry := formattedLog.Front(); logEntry != nil; logEntry = logEntry.Next() {
		commit := logEntry.Value.(*Commit)
		branches, err := l.GetBranches(repo, commit, 2)
		if err != nil {
			return nil, err
		}

		if len(branches) > 1 {
			break
		}

		commits.PushBack(commit)
	}
	return commits, nil
}

// GetBranches returns the branches for this commit in the provided repository
func (LogService) GetBranches(repo service.Repository, commit service.Commit, limit int) ([]string, error) {
	if git.CheckGitVersionAtLeast("2.7.0") == nil {
		stdout, err := git.NewCommand("for-each-ref", "--count="+strconv.Itoa(limit), "--format=%(refname:strip=2)", "--contains", commit.ID().String(), git.BranchPrefix).RunInDir(repo.Path())
		if err != nil {
			return nil, err
		}

		branches := strings.Fields(stdout)
		return branches, nil
	}

	stdout, err := git.NewCommand("branch", "--contains", commit.ID().String()).RunInDir(repo.Path())
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

// FilesCountBetween return the number of files changed between two commits
func (LogService) FilesCountBetween(repo service.Repository, startCommitID, endCommitID string) (int, error) {
	stdout, err := git.NewCommand("diff", "--name-only", startCommitID+"..."+endCommitID).RunInDir(repo.Path())
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// git >= 2.28 now returns an error if startCommitID and endCommitID have become unrelated.
		// previously it would return the results of git diff --name-only startCommitID endCommitID so let's try that...
		stdout, err = git.NewCommand("diff", "--name-only", startCommitID, endCommitID).RunInDir(repo.Path())
	}
	if err != nil {
		return 0, err
	}
	return len(strings.Split(stdout, "\n")) - 1, nil
}

func (LogService) commitsBetweenLimit(repo service.Repository, last, before string, limit, skip int) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	var cmd = git.NewCommand("rev-list")

	if limit > 0 {
		cmd = cmd.AddArguments("--max-count", strconv.Itoa(limit),
			"--skip", strconv.Itoa(skip))
	}

	if before == "" {
		cmd = cmd.AddArguments(last)
	} else {
		cmd = cmd.AddArguments(before + "..." + last)
	}

	go common.PipeCommand(
		cmd,
		repo.Path(),
		stdoutWriter,
		nil)

	commits, err := BatchReadCommits(repo, stdoutReader)
	if err != nil && strings.Contains(err.Error(), "no merge base") && before != "" {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		cmd = git.NewCommand("rev-list")
		if limit > 0 {
			cmd = cmd.AddArguments("--max-count", strconv.Itoa(limit),
				"--skip", strconv.Itoa(skip))
		}
		cmd = cmd.AddArguments(before, last)
		stdoutReader, stdoutWriter := io.Pipe()
		defer func() {
			_ = stdoutReader.Close()
			_ = stdoutWriter.Close()
		}()
		go common.PipeCommand(
			cmd,
			repo.Path(),
			stdoutWriter,
			nil)
		commits, err = BatchReadCommits(repo, stdoutReader)
	}
	return commits, err
}

// CommitsBetween returns a list that contains commits between [last, before).
func (l LogService) CommitsBetween(repo service.Repository, last service.Commit, before service.Commit) (*list.List, error) {
	beforeID := ""
	if before != nil {
		beforeID = before.ID().String()
	}
	return l.commitsBetweenLimit(repo, last.ID().String(), beforeID, -1, -1)
}

// CommitsBetweenLimit returns a list that contains at most limit commits skipping the first skip commits between [last, before)
func (l LogService) CommitsBetweenLimit(repo service.Repository, last service.Commit, before service.Commit, limit, skip int) (*list.List, error) {
	beforeID := ""
	if before != nil {
		beforeID = before.ID().String()
	}
	return l.commitsBetweenLimit(repo, last.ID().String(), beforeID, limit, skip)
}

// CommitsCountBetween return numbers of commits between two commits
func (l LogService) CommitsCountBetween(repo service.Repository, start, end string) (int64, error) {
	count, err := git.CommitsCountFiles(repo.Path(), []string{start + "..." + end}, []string{})
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		return git.CommitsCountFiles(repo.Path(), []string{start, end}, []string{})
	}

	return count, err
}

// CommitsBetweenIDs return commits between two commits
func (l LogService) CommitsBetweenIDs(repo service.Repository, last, before string) (*list.List, error) {
	return l.commitsBetweenLimit(repo, last, before, -1, -1)
}

// GetCommitsFromIDs get commits from commit IDs
func (l LogService) GetCommitsFromIDs(repo service.Repository, commitIDs []string) (*list.List, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	go func() {
		w := bufio.NewWriter(stdoutWriter)
		for _, commitID := range commitIDs {
			_, err := w.WriteString(commitID + "\n")
			if err != nil {
				_ = stdoutWriter.CloseWithError(err)
			}
		}
		err := w.Flush()
		if err != nil {
			_ = stdoutWriter.CloseWithError(err)
		}
		_ = stdoutWriter.Close()
	}()
	return BatchReadCommits(repo, stdoutReader)
}

// GetAllCommitsCount returns count of all commits in repository
func (LogService) GetAllCommitsCount(repo service.Repository) (int64, error) {
	return git.AllCommitsCount(repo.Path(), false)
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func (LogService) GetLatestCommitTime(repoPath string) (time.Time, error) {
	cmd := git.NewCommand("for-each-ref", "--sort=-committerdate", "refs/heads/", "--count", "1", "--format=%(committerdate)")
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse(service.GitTimeLayout, commitTime)
}

// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
func (LogService) GetFullCommitID(repoPath, shortID string) (string, error) {
	commitID, err := git.NewCommand("rev-parse", shortID).RunInDir(repoPath)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 128") {
			return "", git.ErrNotExist{ID: shortID, RelPath: ""}
		}
		return "", err
	}
	return strings.TrimSpace(commitID), nil
}
