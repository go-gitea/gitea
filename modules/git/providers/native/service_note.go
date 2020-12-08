// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"io"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// NoteService represents a note service
type NoteService struct {
}

// GetNote retrieves the git-notes data for a given commit.
func (NoteService) GetNote(repo service.Repository, commitID string) (io.ReadCloser, service.Commit, error) {
	notes, err := repo.GetCommit(git.NotesRef)
	if err != nil {
		return nil, nil, err
	}

	path := ""

	tree := notes.Tree()

	var entry service.TreeEntry
	for len(commitID) > 2 {
		entry, err = tree.GetTreeEntryByPath(commitID)
		if err == nil {
			path += commitID
			break
		}
		if git.IsErrNotExist(err) {
			tree, err = tree.SubTree(commitID[0:2])
			path += commitID[0:2] + "/"
			commitID = commitID[2:]
		}
		if err != nil {
			return nil, nil, err
		}
	}

	reader, err := entry.Reader()
	if err != nil {
		return nil, nil, err
	}

	lastCommits, err := GetLastCommitForPaths(notes, "", []string{path})
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}
	return reader, lastCommits[0], nil
}
