// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Blob) = &Blob{}

// Blob represents a Git object.
type Blob struct {
	service.Object

	name string
}

// NewBlob creates a blob from a provided object and a name
func NewBlob(object service.Object, name string) service.Blob {
	return &Blob{
		Object: object,
		name:   name,
	}
}

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// Type of a Blob is always Blob
func (b *Blob) Type() service.ObjectType {
	return service.ObjectBlob
}
