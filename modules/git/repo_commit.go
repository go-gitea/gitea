// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"fmt"
	"strconv"
	"strings"

	"github.com/mcuadros/go-version"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	ref, err := repo.gogitRepo.Reference(plumbing.ReferenceName(name), true)
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	hash := plumbing.NewHash(name)
	_, err := repo.gogitRepo.CommitObject(hash)
	return err == nil
}

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(name string) (string, error) {
	return repo.GetRefCommitID(BranchPrefix + name)
}

// GetTagCommitID returns last commit ID string of given tag.
func (repo *Repository) GetTagCommitID(name string) (string, error) {
	stdout, err := NewCommand("rev-list", "-n", "1", name).RunInDir(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision or path") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func convertPGPSignatureForTag(t *object.Tag) *CommitGPGSignature {
	if t.PGPSignature == "" {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w,
		"object %s\ntype %s\ntag %s\ntagger ",
		t.Target.String(), t.TargetType.Bytes(), t.Name); err != nil {
		return nil
	}

	if err = t.Tagger.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n"); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, t.Message); err != nil {
		return nil
	}

	return &CommitGPGSignature{
		Signature: t.PGPSignature,
		Payload:   strings.TrimSpace(w.String()) + "\n",
	}
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	var tagObject *object.Tag

	gogitCommit, err := repo.gogitRepo.CommitObject(id)
	if err == plumbing.ErrObjectNotFound {
		tagObject, err = repo.gogitRepo.TagObject(id)
		if err == nil {
			gogitCommit, err = repo.gogitRepo.CommitObject(tagObject.Target)
		}
	}
	if err != nil {
		return nil, err
	}

	commit := convertCommit(gogitCommit)
	commit.repo = repo

	if tagObject != nil {
		commit.CommitMessage = strings.TrimSpace(tagObject.Message)
		commit.Author = &tagObject.Tagger
		commit.Signature = convertPGPSignatureForTag(tagObject)
	}

	tree, err := gogitCommit.Tree()
	if err != nil {
		return nil, err
	}

	commit.Tree.ID = tree.Hash
	commit.Tree.gogitTree = tree

	return commit, nil
}

// ConvertToSHA1 returns a Hash object from a potential ID string
func (repo *Repository) ConvertToSHA1(commitID string) (SHA1, error) {
	if len(commitID) != 40 {
		var err error
		actualCommitID, err := NewCommand("rev-parse", "--verify", commitID).RunInDir(repo.Path)
		if err != nil {
			if strings.Contains(err.Error(), "unknown revision or path") ||
				strings.Contains(err.Error(), "fatal: Needed a single revision") {
				return SHA1{}, ErrNotExist{commitID, ""}
			}
			return SHA1{}, err
		}
		commitID = actualCommitID
	}
	return NewIDFromString(commitID)
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

	stdout, err := NewCommand("log", "-1", prettyLogFormat, id.String(), "--", relpath).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	id, err = NewIDFromString(stdout)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

// GetCommitByPath returns the last commit of relative path.
func (repo *Repository) GetCommitByPath(relpath string) (*Commit, error) {
	stdout, err := NewCommand("log", "-1", prettyLogFormat, "--", relpath).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	commits, err := repo.parsePrettyFormatLogToList(stdout)
	if err != nil {
		return nil, err
	}
	return commits.Front().Value.(*Commit), nil
}

// CommitsRangeSize the default commits range size
var CommitsRangeSize = 50

func (repo *Repository) commitsByRange(id SHA1, page int) (*list.List, error) {
	stdout, err := NewCommand("log", id.String(), "--skip="+strconv.Itoa((page-1)*CommitsRangeSize),
		"--max-count="+strconv.Itoa(CommitsRangeSize), prettyLogFormat).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(stdout)
}

func (repo *Repository) searchCommits(id SHA1, opts SearchCommitsOptions) (*list.List, error) {
	cmd := NewCommand("log", id.String(), "-100", prettyLogFormat)
	args := []string{"-i"}
	if len(opts.Authors) > 0 {
		for _, v := range opts.Authors {
			args = append(args, "--author="+v)
		}
	}
	if len(opts.Committers) > 0 {
		for _, v := range opts.Committers {
			args = append(args, "--committer="+v)
		}
	}
	if len(opts.After) > 0 {
		args = append(args, "--after="+opts.After)
	}
	if len(opts.Before) > 0 {
		args = append(args, "--before="+opts.Before)
	}
	if opts.All {
		args = append(args, "--all")
	}
	if len(opts.Keywords) > 0 {
		for _, v := range opts.Keywords {
			cmd.AddArguments("--grep=" + v)
		}
	}
	cmd.AddArguments(args...)
	stdout, err := cmd.RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	if len(stdout) != 0 {
		stdout = append(stdout, '\n')
	}
	if len(opts.Keywords) > 0 {
		for _, v := range opts.Keywords {
			if len(v) >= 4 {
				hashCmd := NewCommand("log", "-1", prettyLogFormat)
				hashCmd.AddArguments(args...)
				hashCmd.AddArguments(v)
				hashMatching, err := hashCmd.RunInDirBytes(repo.Path)
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

func (repo *Repository) getFilesChanged(id1, id2 string) ([]string, error) {
	stdout, err := NewCommand("diff", "--name-only", id1, id2).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(stdout), "\n"), nil
}

// FileChangedBetweenCommits Returns true if the file changed between commit IDs id1 and id2
// You must ensure that id1 and id2 are valid commit ids.
func (repo *Repository) FileChangedBetweenCommits(filename, id1, id2 string) (bool, error) {
	stdout, err := NewCommand("diff", "--name-only", "-z", id1, id2, "--", filename).RunInDirBytes(repo.Path)
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(stdout))) > 0, nil
}

// FileCommitsCount return the number of files at a revison
func (repo *Repository) FileCommitsCount(revision, file string) (int64, error) {
	return commitsCount(repo.Path, revision, file)
}

// CommitsByFileAndRange return the commits according revison file and the page
func (repo *Repository) CommitsByFileAndRange(revision, file string, page int) (*list.List, error) {
	stdout, err := NewCommand("log", revision, "--follow", "--skip="+strconv.Itoa((page-1)*50),
		"--max-count="+strconv.Itoa(CommitsRangeSize), prettyLogFormat, "--", file).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(stdout)
}

// CommitsByFileAndRangeNoFollow return the commits according revison file and the page
func (repo *Repository) CommitsByFileAndRangeNoFollow(revision, file string, page int) (*list.List, error) {
	stdout, err := NewCommand("log", revision, "--skip="+strconv.Itoa((page-1)*50),
		"--max-count="+strconv.Itoa(CommitsRangeSize), prettyLogFormat, "--", file).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(stdout)
}

// FilesCountBetween return the number of files changed between two commits
func (repo *Repository) FilesCountBetween(startCommitID, endCommitID string) (int, error) {
	stdout, err := NewCommand("diff", "--name-only", startCommitID+"..."+endCommitID).RunInDir(repo.Path)
	if err != nil {
		return 0, err
	}
	return len(strings.Split(stdout, "\n")) - 1, nil
}

// CommitsBetween returns a list that contains commits between [last, before).
func (repo *Repository) CommitsBetween(last *Commit, before *Commit) (*list.List, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, err = NewCommand("rev-list", last.ID.String()).RunInDirBytes(repo.Path)
	} else {
		stdout, err = NewCommand("rev-list", before.ID.String()+"..."+last.ID.String()).RunInDirBytes(repo.Path)
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
}

// CommitsBetweenLimit returns a list that contains at most limit commits skipping the first skip commits between [last, before)
func (repo *Repository) CommitsBetweenLimit(last *Commit, before *Commit, limit, skip int) (*list.List, error) {
	var stdout []byte
	var err error
	if before == nil {
		stdout, err = NewCommand("rev-list", "--max-count", strconv.Itoa(limit), "--skip", strconv.Itoa(skip), last.ID.String()).RunInDirBytes(repo.Path)
	} else {
		stdout, err = NewCommand("rev-list", "--max-count", strconv.Itoa(limit), "--skip", strconv.Itoa(skip), before.ID.String()+"..."+last.ID.String()).RunInDirBytes(repo.Path)
	}
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
}

// CommitsBetweenIDs return commits between twoe commits
func (repo *Repository) CommitsBetweenIDs(last, before string) (*list.List, error) {
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
	return commitsCount(repo.Path, start+"..."+end, "")
}

// commitsBefore the limit is depth, not total number of returned commits.
func (repo *Repository) commitsBefore(id SHA1, limit int) (*list.List, error) {
	cmd := NewCommand("log")
	if limit > 0 {
		cmd.AddArguments("-"+strconv.Itoa(limit), prettyLogFormat, id.String())
	} else {
		cmd.AddArguments(prettyLogFormat, id.String())
	}

	stdout, err := cmd.RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	formattedLog, err := repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}

	commits := list.New()
	for logEntry := formattedLog.Front(); logEntry != nil; logEntry = logEntry.Next() {
		commit := logEntry.Value.(*Commit)
		branches, err := repo.getBranches(commit, 2)
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

func (repo *Repository) getCommitsBefore(id SHA1) (*list.List, error) {
	return repo.commitsBefore(id, 0)
}

func (repo *Repository) getCommitsBeforeLimit(id SHA1, num int) (*list.List, error) {
	return repo.commitsBefore(id, num)
}

func (repo *Repository) getBranches(commit *Commit, limit int) ([]string, error) {
	if version.Compare(gitVersion, "2.7.0", ">=") {
		stdout, err := NewCommand("for-each-ref", "--count="+strconv.Itoa(limit), "--format=%(refname:strip=2)", "--contains", commit.ID.String(), BranchPrefix).RunInDir(repo.Path)
		if err != nil {
			return nil, err
		}

		branches := strings.Fields(stdout)
		return branches, nil
	}

	stdout, err := NewCommand("branch", "--contains", commit.ID.String()).RunInDir(repo.Path)
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
