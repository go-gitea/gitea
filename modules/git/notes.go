// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"path"
	"strings"

	"gitea.dev/modules/setting"
)

// NotesRef is the git ref where Gitea will look for git-notes data.
// The value ("refs/notes/commits") is the default ref used by git-notes.
const NotesRef = "refs/notes/commits"

// Note stores information about a note created using git-notes.
type Note struct {
	refCommit *Commit

	BlobMessage CommitMessage // if the blob is too large, the message will be truncated
	BlobSize    int64
	TreePath    string
}

// GetNote retrieves the git-notes data for a given commit.
func GetNote(ctx context.Context, repo *Repository, commitID string) (*Note, error) {
	noteCommit, err := repo.GetCommit(ctx, NotesRef)
	if err != nil {
		return nil, err
	}

	// A note for a commit is stored in a blob in the notes commit tree, with the path being the commit ID.
	// The path can be "FullCommitID" or a fanout path like "ab/cdef...." or "ab/cd/ef.....".
	tree := noteCommit.Tree()
	entryName := commitID
	var entry *TreeEntry
	var treePathBuf strings.Builder
	for len(entryName) > 2 {
		entry, err = tree.GetTreeEntryByPath(ctx, repo, entryName)
		if err == nil {
			treePathBuf.WriteString(entryName)
			break
		} else if IsErrNotExist(err) {
			fanoutDir, fanoutName := entryName[0:2], entryName[2:]
			tree, err = tree.SubTree(ctx, repo, fanoutDir)
			if err != nil {
				return nil, err
			}
			treePathBuf.WriteString(fanoutDir)
			treePathBuf.WriteByte('/')
			entryName = fanoutName
		} else {
			return nil, err
		}
	}
	if entry == nil {
		return nil, ErrNotExist{ID: commitID}
	}

	treePath := treePathBuf.String()
	blob := entry.Blob(repo)
	note := &Note{TreePath: treePath, refCommit: noteCommit}
	note.BlobMessage.MessageRaw, err = blob.GetBlobContent(ctx, setting.UI.MaxDisplayFileSize)
	if err != nil {
		return nil, err
	}
	note.BlobSize = blob.Size(ctx) // it should be called after the get blob content, then the "size" is cached
	return note, nil
}

func GetNoteWithLastCommit(ctx context.Context, repo *Repository, commitID string) (*Note, *Commit, error) {
	note, err := GetNote(ctx, repo, commitID)
	if err != nil {
		return nil, nil, err
	}
	parentPath, entryName := path.Split(note.TreePath)
	parentPath = strings.Trim(parentPath, "/")
	lastCommits, err := GetLastCommitForPaths(ctx, repo, note.refCommit, parentPath, []string{entryName})
	if err != nil {
		return nil, nil, err
	}
	lastCommit := lastCommits[entryName]
	if lastCommit == nil {
		return nil, nil, ErrNotExist{ID: commitID}
	}
	return note, lastCommit, nil
}
