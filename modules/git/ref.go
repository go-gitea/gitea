// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "strings"

// Reference represents a Git ref.
type Reference struct {
	Name   string
	repo   *Repository
	Object SHA1 // The id of this commit object
	Type   string
}

// Commit return the commit of the reference
func (ref *Reference) Commit() (*Commit, error) {
	return ref.repo.getCommit(ref.Object)
}

// ShortName returns the short name of the reference
func (ref *Reference) ShortName() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.Name, "refs/heads/") {
		return ref.Name[11:]
	}
	if strings.HasPrefix(ref.Name, "refs/tags/") {
		return ref.Name[10:]
	}
	if strings.HasPrefix(ref.Name, "refs/remotes/") {
		return ref.Name[13:]
	}
	if strings.HasPrefix(ref.Name, "refs/pull/") && strings.IndexByte(ref.Name[10:], '/') > -1 {
		return ref.Name[10 : strings.IndexByte(ref.Name[10:], '/')+10]
	}

	return ref.Name
}

// RefGroup returns the group type of the reference
func (ref *Reference) RefGroup() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.Name, "refs/heads/") {
		return "heads"
	}
	if strings.HasPrefix(ref.Name, "refs/tags/") {
		return "tags"
	}
	if strings.HasPrefix(ref.Name, "refs/remotes/") {
		return "remotes"
	}
	if strings.HasPrefix(ref.Name, "refs/pull/") && strings.IndexByte(ref.Name[10:], '/') > -1 {
		return "pull"
	}
	return ""
}
