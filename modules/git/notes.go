// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"

	"gopkg.in/src-d/go-git.v4/plumbing"
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

	entry, err := notes.GetTreeEntryByPath(commitID)
	if err != nil {
		return err
	}

	blob := entry.Blob()
	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}

	defer dataRc.Close()
	d, err := ioutil.ReadAll(dataRc)
	if err != nil {
		return err
	}
	note.Message = d

	commit, err := repo.gogitRepo.CommitObject(plumbing.Hash(notes.ID))
	if err != nil {
		return err
	}

	lastCommits, err := getLastCommitForPaths(commit, "", []string{commitID})
	if err != nil {
		return err
	}
	note.Commit = convertCommit(lastCommits[commitID])

	return nil
}
