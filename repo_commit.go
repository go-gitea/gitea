// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"strings"
)

// getCommitIDOfRef returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) getCommitIDOfRef(refName string) (string, error) {
	stdout, err := NewCommand("show-ref", "--verify", refName).RunInDir(repo.Path)
	if err != nil {
		return "", err
	}
	return strings.Split(stdout, " ")[0], nil
}

// GetCommitIDOfBranch returns last commit ID string of given branch.
func (repo *Repository) GetCommitIDOfBranch(branch string) (string, error) {
	return repo.getCommitIDOfRef(BRANCH_PREFIX + branch)
}

// parseCommitData parses commit information from the (uncompressed) raw
// data from the commit object.
// \n\n separate headers from message
func parseCommitData(data []byte) (*Commit, error) {
	commit := new(Commit)
	commit.parents = make([]sha1, 0, 1)
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
			case "tree":
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
			case "author":
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

func (repo *Repository) getCommit(id sha1) (*Commit, error) {
	if repo.commitCache != nil {
		log("Hit cache: %s", id)
		if c, ok := repo.commitCache[id]; ok {
			return c, nil
		}
	} else {
		repo.commitCache = make(map[sha1]*Commit, 10)
	}

	data, err := NewCommand("cat-file", "-p", id.String()).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	commit, err := parseCommitData(data)
	if err != nil {
		return nil, err
	}
	commit.repo = repo
	commit.ID = id

	repo.commitCache[id] = commit
	return commit, nil
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(commitID string) (*Commit, error) {
	id, err := NewIDFromString(commitID)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

// GetCommitOfBranch returns the last commit of given branch.
func (repo *Repository) GetCommitOfBranch(branch string) (*Commit, error) {
	commitID, err := repo.GetCommitIDOfBranch(branch)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(commitID)
}

func (repo *Repository) getCommitOfRelPath(id sha1, relpath string) (*Commit, error) {
	stdout, err := NewCommand("log", "-1", _PRETTY_LOG_FORMAT, id.String(), "--", relpath).RunInDir(repo.Path)
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
	stdout, err := NewCommand("log", "-1", _PRETTY_LOG_FORMAT, "--", relpath).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	commits, err := repo.parsePrettyFormatLogToList(stdout)
	if err != nil {
		return nil, err
	}
	return commits.Front().Value.(*Commit), nil
}
