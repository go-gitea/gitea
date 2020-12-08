// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"fmt"
	"io"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// NoteService represents a note service
type NoteService struct{}

// GetNote retrieves the git-notes data for a given commit.
func (NoteService) GetNote(repo service.Repository, commitID string) (io.ReadCloser, service.Commit, error) {
	gogitRepo, ok := repo.(*Repository)
	if !ok {
		return nil, nil, fmt.Errorf("Not a gogit repository")
	}

	notes, err := repo.GetCommit(git.NotesRef)
	if err != nil {
		return nil, nil, err
	}

	remainingCommitID := commitID
	path := ""

	notesTree, ok := notes.Tree().(*Tree)
	if !ok {
		return nil, nil, fmt.Errorf("Not a gogit repository")
	}

	currentTree := notesTree.gogitTree
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
			return nil, nil, err
		}
	}

	blob := file.Blob
	reader, err := blob.Reader()
	if err != nil {
		return nil, nil, err
	}

	commitNodeIndex, commitGraphFile := gogitRepo.CommitNodeIndex()
	if commitGraphFile != nil {
		defer commitGraphFile.Close()
	}

	commitNode, err := commitNodeIndex.Get(ToPlumbingHash(notes.ID()))
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}

	lastCommits, err := GetLastCommitForPaths(commitNode, "", []string{path})
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}
	return reader, convertCommit(repo, lastCommits[path]), nil
}
