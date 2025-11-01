// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/log"
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

	tree := &notes.Tree
	log.Trace("Found tree with ID %q while searching for git note corresponding to the commit %q", tree.ID, commitID)

	var entry *TreeEntry
	originalCommitID := commitID
	var pathSb strings.Builder
	for len(commitID) > 2 {
		entry, err = tree.GetTreeEntryByPath(commitID)
		if err == nil {
			pathSb.WriteString(commitID)
			break
		}
		if IsErrNotExist(err) {
			tree, err = tree.SubTree(commitID[0:2])
			pathSb.WriteString(commitID[0:2] + "/")
			commitID = commitID[2:]
		}
		if err != nil {
			// Err may have been updated by the SubTree we need to recheck if it's again an ErrNotExist
			if !IsErrNotExist(err) {
				log.Error("Unable to find git note corresponding to the commit %q. Error: %v", originalCommitID, err)
			}
			return err
		}
	}
	path := pathSb.String()

	blob := entry.Blob()
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Error("Unable to read blob with ID %q. Error: %v", blob.ID, err)
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = dataRc.Close()
		}
	}()
	d, err := io.ReadAll(dataRc)
	if err != nil {
		log.Error("Unable to read blob with ID %q. Error: %v", blob.ID, err)
		return err
	}
	_ = dataRc.Close()
	closed = true
	note.Message = d

	treePath := ""
	if idx := strings.LastIndex(path, "/"); idx > -1 {
		treePath = path[:idx]
		path = path[idx+1:]
	}

	lastCommits, err := GetLastCommitForPaths(ctx, notes, treePath, []string{path})
	if err != nil {
		log.Error("Unable to get the commit for the path %q. Error: %v", treePath, err)
		return err
	}
	note.Commit = lastCommits[path]

	return nil
}
