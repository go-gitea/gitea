// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"strconv"
	"strings"

	version "github.com/mcuadros/go-version"
)

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	stdout, err := NewCommand("show-ref", "--verify", name).RunInDir(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	return strings.Split(stdout, " ")[0], nil
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

// parseCommitData parses commit information from the (uncompressed) raw
// data from the commit object.
// \n\n separate headers from message
func parseCommitData(data []byte) (*Commit, error) {
	commit := new(Commit)
	commit.parents = make([]SHA1, 0, 1)
	// we now have the contents of the commit object. Let's investigate...
	nextline := 0
l:
	for {
		eol := bytes.IndexByte(data[nextline:], '\n')
		switch {
		case eol > 0:
			line := data[nextline : nextline+eol]
			spacepos := bytes.IndexByte(line, ' ')
			reftype := line[:spacepos]
			switch string(reftype) {
			case "tree", "object":
				id, err := NewIDFromString(string(line[spacepos+1:]))
				if err != nil {
					return nil, err
				}
				commit.Tree.ID = id
			case "parent":
				// A commit can have one or more parents
				oid, err := NewIDFromString(string(line[spacepos+1:]))
				if err != nil {
					return nil, err
				}
				commit.parents = append(commit.parents, oid)
			case "author", "tagger":
				sig, err := newSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				commit.Author = sig
			case "committer":
				sig, err := newSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				commit.Committer = sig
			case "gpgsig":
				sig, err := newGPGSignatureFromCommitline(data, nextline+spacepos+1, false)
				if err != nil {
					return nil, err
				}
				commit.Signature = sig
			}
			nextline += eol + 1
		case eol == 0:
			cm := string(data[nextline+1:])

			// Tag GPG signatures are stored below the commit message
			sigindex := strings.Index(cm, "-----BEGIN PGP SIGNATURE-----")
			if sigindex != -1 {
				sig, err := newGPGSignatureFromCommitline(data, (nextline+1)+sigindex, true)
				if err == nil && sig != nil {
					// remove signature from commit message
					if sigindex == 0 {
						cm = ""
					} else {
						cm = cm[:sigindex-1]
					}
					commit.Signature = sig
				}
			}

			commit.CommitMessage = cm
			break l
		default:
			break l
		}
	}
	return commit, nil
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	c, ok := repo.commitCache.Get(id.String())
	if ok {
		log("Hit cache: %s", id)
		return c.(*Commit), nil
	}

	data, err := NewCommand("cat-file", "-p", id.String()).RunInDirBytes(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") {
			return nil, ErrNotExist{id.String(), ""}
		}
		return nil, err
	}

	commit, err := parseCommitData(data)
	if err != nil {
		return nil, err
	}
	commit.repo = repo
	commit.ID = id

	data, err = NewCommand("name-rev", id.String()).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	// name-rev commitID ouput will be "COMMIT_ID master" or "COMMIT_ID master~12"
	commit.Branch = strings.Split(strings.Split(string(data), " ")[1], "~")[0]

	repo.commitCache.Set(id.String(), commit)
	return commit, nil
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(commitID string) (*Commit, error) {
	if len(commitID) != 40 {
		var err error
		actualCommitID, err := NewCommand("rev-parse", commitID).RunInDir(repo.Path)
		if err != nil {
			if strings.Contains(err.Error(), "unknown revision or path") {
				return nil, ErrNotExist{commitID, ""}
			}
			return nil, err
		}
		commitID = actualCommitID
	}
	id, err := NewIDFromString(commitID)
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

func (repo *Repository) searchCommits(id SHA1, keyword string, all bool) (*list.List, error) {
	cmd := NewCommand("log", id.String(), "-100", "-i", "--grep="+keyword, prettyLogFormat)
	if all {
		cmd.AddArguments("--all")
	}
	stdout, err := cmd.RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return repo.parsePrettyFormatLogToList(stdout)
}

func (repo *Repository) getFilesChanged(id1 string, id2 string) ([]string, error) {
	stdout, err := NewCommand("diff", "--name-only", id1, id2).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(stdout), "\n"), nil
}

// FileCommitsCount return the number of files at a revison
func (repo *Repository) FileCommitsCount(revision, file string) (int64, error) {
	return commitsCount(repo.Path, revision, file)
}

// CommitsByFileAndRange return the commits accroding revison file and the page
func (repo *Repository) CommitsByFileAndRange(revision, file string, page int) (*list.List, error) {
	stdout, err := NewCommand("log", revision, "--follow", "--skip="+strconv.Itoa((page-1)*50),
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
	stdout, err := NewCommand("rev-list", before.ID.String()+"..."+last.ID.String()).RunInDirBytes(repo.Path)
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
