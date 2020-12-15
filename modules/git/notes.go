// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"io/ioutil"

	"code.gitea.io/gitea/modules/git/service"
)

// NotesRef is the git ref where Gitea will look for git-notes data.
// The value ("refs/notes/commits") is the default ref used by git-notes.
const NotesRef = "refs/notes/commits"

// Note stores information about a note created using git-notes.
type Note struct {
	Message []byte
	Commit  service.Commit
}

// GetNote retrieves the git-notes data for a given commit.
func GetNote(repo service.Repository, commitID string) (*Note, error) {
	reader, commit, err := Service.GetNote(repo, commitID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	d, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	note := &Note{}
	note.Message = d
	note.Commit = commit

	return note, nil
}
