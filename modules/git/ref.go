// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "strings"

const (
	// RemotePrefix is the base directory of the remotes information of git.
	RemotePrefix = "refs/remotes/"
	// PullPrefix is the base directory of the pull information of git.
	PullPrefix = "refs/pull/"

	pullLen = len(PullPrefix)
)

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
	if strings.HasPrefix(ref.Name, BranchPrefix) {
		return strings.TrimPrefix(ref.Name, BranchPrefix)
	}
	if strings.HasPrefix(ref.Name, TagPrefix) {
		return strings.TrimPrefix(ref.Name, TagPrefix)
	}
	if strings.HasPrefix(ref.Name, RemotePrefix) {
		return strings.TrimPrefix(ref.Name, RemotePrefix)
	}
	if strings.HasPrefix(ref.Name, PullPrefix) && strings.IndexByte(ref.Name[pullLen:], '/') > -1 {
		return ref.Name[pullLen : strings.IndexByte(ref.Name[pullLen:], '/')+pullLen]
	}

	return ref.Name
}

// RefGroup returns the group type of the reference
func (ref *Reference) RefGroup() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.Name, BranchPrefix) {
		return "heads"
	}
	if strings.HasPrefix(ref.Name, TagPrefix) {
		return "tags"
	}
	if strings.HasPrefix(ref.Name, RemotePrefix) {
		return "remotes"
	}
	if strings.HasPrefix(ref.Name, PullPrefix) && strings.IndexByte(ref.Name[pullLen:], '/') > -1 {
		return "pull"
	}
	return ""
}
