// Copyright 2015 The Gogs Authors. All rights reserved.
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
)

// getRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) getRefCommitID(name string) (string, error) {
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
	return repo.getRefCommitID(BranchPrefix + name)
}

// GetTagCommitID returns last commit ID string of given tag.
func (repo *Repository) GetTagCommitID(name string) (string, error) {
	return repo.getRefCommitID(TagPrefix + name)
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
				sig, err := newGPGSignatureFromCommitline(data, nextline+spacepos+1)
				if err != nil {
					return nil, err
				}
				commit.Signature = sig
			}
			nextline += eol + 1
		case eol == 0:
			commit.CommitMessage = string(data[nextline+1:])
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

	repo.commitCache.Set(id.String(), commit)
	return commit, nil
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(commitID string) (*Commit, error) {
	if len(commitID) != 40 {
		var err error
		commitID, err = NewCommand("rev-parse", commitID).RunInDir(repo.Path)
		if err != nil {
			return nil, err
		}
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
	if version.Compare(gitVersion, "1.8.0", ">=") {
		stdout, err := NewCommand("rev-list", before.ID.String()+"..."+last.ID.String()).RunInDirBytes(repo.Path)
		if err != nil {
			return nil, err
		}
		return repo.parsePrettyFormatLogToList(bytes.TrimSpace(stdout))
	}

	// Fallback to stupid solution, which iterates all commits of the repository
	// if before is not an ancestor of last.
	l := list.New()
	if last == nil || last.ParentCount() == 0 {
		return l, nil
	}

	var err error
	cur := last
	for {
		if cur.ID.Equal(before.ID) {
			break
		}
		l.PushBack(cur)
		if cur.ParentCount() == 0 {
			break
		}
		cur, err = cur.Parent(0)
		if err != nil {
			return nil, err
		}
	}
	return l, nil
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
func (repo *Repository) commitsBefore(l *list.List, parent *list.Element, id SHA1, current, limit int) error {
	// Reach the limit
	if limit > 0 && current > limit {
		return nil
	}

	commit, err := repo.getCommit(id)
	if err != nil {
		return fmt.Errorf("getCommit: %v", err)
	}

	var e *list.Element
	if parent == nil {
		e = l.PushBack(commit)
	} else {
		var in = parent
		for {
			if in == nil {
				break
			} else if in.Value.(*Commit).ID.Equal(commit.ID) {
				return nil
			} else if in.Next() == nil {
				break
			}

			if in.Value.(*Commit).Committer.When.Equal(commit.Committer.When) {
				break
			}

			if in.Value.(*Commit).Committer.When.After(commit.Committer.When) &&
				in.Next().Value.(*Commit).Committer.When.Before(commit.Committer.When) {
				break
			}

			in = in.Next()
		}

		e = l.InsertAfter(commit, in)
	}

	pr := parent
	if commit.ParentCount() > 1 {
		pr = e
	}

	for i := 0; i < commit.ParentCount(); i++ {
		id, err := commit.ParentID(i)
		if err != nil {
			return err
		}
		err = repo.commitsBefore(l, pr, id, current+1, limit)
		if err != nil {
			return err
		}
	}

	return nil
}

func (repo *Repository) getCommitsBefore(id SHA1) (*list.List, error) {
	l := list.New()
	return l, repo.commitsBefore(l, nil, id, 1, 0)
}

func (repo *Repository) getCommitsBeforeLimit(id SHA1, num int) (*list.List, error) {
	l := list.New()
	return l, repo.commitsBefore(l, nil, id, 1, num)
}
