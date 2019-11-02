// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// NotesRef is the git ref where Gitea will look for git-notes data.
// The value ("refs/notes/commits") is the default ref used by git-notes.
const NotesRef = "refs/notes/commits"

// Note stores information about a note created using git-notes.
type Note struct {
	Message []byte
	Commit  *Commit
}

// GetNote retrieves the git-notes data for a given commit.
func GetNote(repo *Repository, commitID string, note *Note) error {
	notes, err := repo.GetCommit(NotesRef)
	if err != nil {
		return err
	}

	remainingCommitID := commitID
	path := ""
	currentTree := notes.Tree.gogitTree
	var file *object.File
	for len(remainingCommitID) > 2 {
		file, err = currentTree.File(remainingCommitID)
		if err == nil {
			path += remainingCommitID
			break
		}
		if err == object.ErrFileNotFound {
			currentTree, err = currentTree.Tree(remainingCommitID[0:2])
			path += remainingCommitID[0:2] + "/"
			remainingCommitID = remainingCommitID[2:]
		}
		if err != nil {
			return err
		}
	}

	blob := file.Blob
	dataRc, err := blob.Reader()
	if err != nil {
		return err
	}

	defer dataRc.Close()
	d, err := ioutil.ReadAll(dataRc)
	if err != nil {
		return err
	}
	note.Message = d

	commitNodeIndex, commitGraphFile := repo.CommitNodeIndex()
	if commitGraphFile != nil {
		defer commitGraphFile.Close()
	}

	commitNode, err := commitNodeIndex.Get(notes.ID)
	if err != nil {
		return err
	}

	lastCommits, err := getLastCommitForPaths(commitNode, "", []string{path})
	if err != nil {
		return err
	}
	note.Commit = convertCommit(lastCommits[path])

	return nil
}
