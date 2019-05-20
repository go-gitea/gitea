// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"
)

// Note stores information about a note created using git-notes.
type Note struct {
	Message []byte
}

// GetNote retrieves the git-notes data for a given commit.
func GetNote(repo *Repository, commitID string, note *Note) error {
	notes, err := repo.GetCommit("refs/notes/commits")
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
	return nil
}
