// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"strings"

	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Reference) = &Reference{}

// Reference represents a Git ref.
type Reference struct {
	name string
	hash service.Hash
	typ  service.ObjectType
	repo service.Repository
}

// NewReference is a constructor for making an service.Reference
func NewReference(name string, hash service.Hash, typ service.ObjectType, repo service.Repository) service.Reference {
	return &Reference{
		name: name,
		hash: hash,
		typ:  typ,
		repo: repo,
	}
}

// Name returns the name of this reference
func (ref *Reference) Name() string {
	return ref.name
}

// ID returns the Hash for this reference
func (ref *Reference) ID() service.Hash {
	return ref.hash
}

// Type returns the git object type of this reference
func (ref *Reference) Type() service.ObjectType {
	return ref.typ
}

// ShortName returns the short name of the reference
func (ref *Reference) ShortName() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.name, "refs/heads/") {
		return ref.name[11:]
	}
	if strings.HasPrefix(ref.name, "refs/tags/") {
		return ref.name[10:]
	}
	if strings.HasPrefix(ref.name, "refs/remotes/") {
		return ref.name[13:]
	}
	if strings.HasPrefix(ref.name, "refs/pull/") && strings.IndexByte(ref.name[10:], '/') > -1 {
		return ref.name[10 : strings.IndexByte(ref.name[10:], '/')+10]
	}

	return ref.name
}

// RefGroup returns the group type of the reference
func (ref *Reference) RefGroup() string {
	if ref == nil {
		return ""
	}
	if strings.HasPrefix(ref.name, "refs/heads/") {
		return "heads"
	}
	if strings.HasPrefix(ref.name, "refs/tags/") {
		return "tags"
	}
	if strings.HasPrefix(ref.name, "refs/remotes/") {
		return "remotes"
	}
	if strings.HasPrefix(ref.name, "refs/pull/") && strings.IndexByte(ref.name[10:], '/') > -1 {
		return "pull"
	}
	return ""
}

// Commit return the commit of the reference
func (ref *Reference) Commit() (service.Commit, error) {
	return ref.repo.GetCommit(ref.hash.String())
}
