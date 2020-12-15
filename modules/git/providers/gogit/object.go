// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var _ (service.Object) = &Object{}

// Object represents a git type
type Object struct {
	hash service.Hash
	repo service.Repository

	encodedObject plumbing.EncodedObject
}

// ID returns the hash this object is associated with
func (o *Object) ID() service.Hash {
	return o.hash
}

// Reader returns a ReadCloser
func (o *Object) Reader() (io.ReadCloser, error) {
	return o.ReaderContext(git.DefaultContext)
}

func (o *Object) loadEncodedObject() error {
	if o.encodedObject != nil {
		return nil
	}
	var gogitRepo *gogit.Repository
	var err error
	gogitRepo, err = GetGoGitRepo(o.repo)
	if err != nil {
		return err
	}

	o.encodedObject, err = gogitRepo.Storer.EncodedObject(plumbing.AnyObject, ToPlumbingHash(o.hash))
	if err != nil {
		return git.ErrNotExist{
			ID:      o.hash.String(),
			RelPath: "",
		}
	}
	return nil
}

// ReaderContext returns a ReadCloser
func (o *Object) ReaderContext(ctx context.Context) (io.ReadCloser, error) {
	if o.encodedObject == nil {
		err := o.loadEncodedObject()
		if err != nil {
			return nil, err
		}
	}
	return o.encodedObject.Reader()
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	if o.encodedObject == nil {
		err := o.loadEncodedObject()
		if err != nil {
			return 0
		}
	}
	return o.encodedObject.Size()
}

// Type returns the type of the object
func (o *Object) Type() service.ObjectType {
	if o.encodedObject == nil {
		err := o.loadEncodedObject()
		if err != nil {
			return service.ObjectType("unknown")
		}
	}

	return service.ObjectType(o.encodedObject.Type().String())
}

// Repository returns the repository this object is associated with
func (o *Object) Repository() service.Repository {
	return o.repo
}
