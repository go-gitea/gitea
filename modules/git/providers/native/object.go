// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

var _ (service.Object) = &Object{}

// Object represents a git type
type Object struct {
	hash service.Hash
	repo service.Repository

	gotSize bool
	size    int64

	gotType bool
	typ     string
}

// NewObject creates an object from hash and repository
func NewObject(hash service.Hash, repo service.Repository) service.Object {
	return &Object{
		hash: hash,
		repo: repo,
	}
}

// ID returns the hash this object is associated with
func (o *Object) ID() service.Hash {
	return o.hash
}

// Reader returns a ReadCloser
func (o *Object) Reader() (io.ReadCloser, error) {
	return o.ReaderContext(git.DefaultContext)
}

// ReaderContext returns a ReadCloser
func (o *Object) ReaderContext(ctx context.Context) (io.ReadCloser, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	var err error

	go func() {
		stderr := &strings.Builder{}
		err = git.NewCommandContext(ctx, "cat-file", "--batch").RunInDirFullPipeline(o.repo.Path(), stdoutWriter, stderr, strings.NewReader(o.hash.String()+"\n"))
		if err != nil {
			err = git.ConcatenateError(err, stderr.String())
			_ = stdoutWriter.CloseWithError(err)
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	bufReader := bufio.NewReader(stdoutReader)
	_, typ, size, err := ReadBatchLine(bufReader)
	if err != nil {
		stdoutReader.Close()
		return nil, err
	}
	o.size = size
	o.gotSize = true
	o.typ = typ
	o.gotType = true

	return &git.LimitedReaderCloser{
		R: bufReader,
		C: stdoutReader,
		N: int64(size),
	}, err
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	if o.gotSize {
		return o.size
	}

	size, err := git.NewCommand("cat-file", "-s", o.hash.String()).RunInDir(o.repo.Path())
	if err != nil {
		log.Error("error whilst reading size for %s in %s. Error: %v", o.hash.String(), o.repo.Path(), err)
		return 0
	}

	o.size, err = strconv.ParseInt(size[:len(size)-1], 10, 64)
	if err != nil {
		log.Error("error whilst parsing size %s for %s in %s. Error: %v", size, o.hash.String(), o.repo.Path(), err)
		return 0
	}
	o.gotSize = true

	return o.size
}

// Type returns the type of the object
func (o *Object) Type() service.ObjectType {
	if o.gotType {
		return service.ObjectType(o.typ)
	}

	var err error
	o.typ, err = git.NewCommand("cat-file", "-t", o.hash.String()).RunInDir(o.repo.Path())
	if err != nil {
		log.Error("error whilst reading size for %s in %s. Error: %v", o.hash.String(), o.repo.Path(), err)
		return ""
	}

	o.gotType = true

	return service.ObjectType(o.typ)
}

// Repository returns the repository this object is associated with
func (o *Object) Repository() service.Repository {
	return o.repo
}
