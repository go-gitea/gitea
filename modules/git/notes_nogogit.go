// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"io/ioutil"
)

// GetNote retrieves the git-notes data for a given commit.
func GetNote(repo *Repository, commitID string, note *Note) error {
	notes, err := repo.GetCommit(NotesRef)
	if err != nil {
		return err
	}

	path := ""

	tree := &notes.Tree

	var entry *TreeEntry
	for len(commitID) > 2 {
		entry, err = tree.GetTreeEntryByPath(commitID)
		if err == nil {
			path += commitID
			break
		}
		if IsErrNotExist(err) {
			tree, err = tree.SubTree(commitID[0:2])
			path += commitID[0:2] + "/"
			commitID = commitID[2:]
		}
		if err != nil {
			return err
		}
	}

	dataRc, err := entry.Blob().DataAsync()
	if err != nil {
		return err
	}
	defer dataRc.Close()
	d, err := ioutil.ReadAll(dataRc)
	if err != nil {
		return err
	}
	note.Message = d

	lastCommits, err := GetLastCommitForPaths(notes, "", []string{path})
	if err != nil {
		return err
	}
	note.Commit = lastCommits[0]

	return nil
}
