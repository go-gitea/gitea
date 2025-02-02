// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetNote retrieves the git-notes data for a given commit.
// FIXME: Add LastCommitCache support
func GetNote(ctx context.Context, repo *Repository, commitID string, note *Note) error {
	log.Trace("Searching for git note corresponding to the commit %q in the repository %q", commitID, repo.Path)
	notes, err := repo.GetCommit(NotesRef)
	if err != nil {
		if IsErrNotExist(err) {
			return err
		}
		log.Error("Unable to get commit from ref %q. Error: %v", NotesRef, err)
		return err
	}

	remainingCommitID := commitID
	path := ""
	currentTree := notes.Tree.gogitTree
	log.Trace("Found tree with ID %q while searching for git note corresponding to the commit %q", currentTree.Entries[0].Name, commitID)
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
			if err == object.ErrDirectoryNotFound {
				return ErrNotExist{ID: remainingCommitID, RelPath: path}
			}
			log.Error("Unable to find git note corresponding to the commit %q. Error: %v", commitID, err)
			return err
		}
	}

	blob := file.Blob
	dataRc, err := blob.Reader()
	if err != nil {
		log.Error("Unable to read blob with ID %q. Error: %v", blob.ID, err)
		return err
	}

	defer dataRc.Close()
	d, err := io.ReadAll(dataRc)
	if err != nil {
		log.Error("Unable to read blob with ID %q. Error: %v", blob.ID, err)
		return err
	}
	note.Message = d

	commitNodeIndex, commitGraphFile := repo.CommitNodeIndex()
	if commitGraphFile != nil {
		defer commitGraphFile.Close()
	}

	commitNode, err := commitNodeIndex.Get(plumbing.Hash(notes.ID.RawValue()))
	if err != nil {
		return err
	}

	lastCommits, err := GetLastCommitForPaths(ctx, nil, commitNode, "", []string{path})
	if err != nil {
		log.Error("Unable to get the commit for the path %q. Error: %v", path, err)
		return err
	}
	note.Commit = lastCommits[path]

	return nil
}
