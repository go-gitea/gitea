// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

import "io"

// NoteService represents a note service
type NoteService interface {
	// GetNote retrieves the git-notes data for a given commit.
	GetNote(repo Repository, commitID string) (io.ReadCloser, Commit, error)
}
