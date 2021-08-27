// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// NotesRef is the git ref where Gitea will look for git-notes data.
// The value ("refs/notes/commits") is the default ref used by git-notes.
const NotesRef = "refs/notes/commits"

// Note stores information about a note created using git-notes.
type Note struct {
	Message []byte
	Commit  *Commit
}
