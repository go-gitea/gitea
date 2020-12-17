// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"errors"
	"path/filepath"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	gpgSettings *GPGSettings
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
	}
	return &Repository{
		Path:     repoPath,
		tagCache: newObjectCache(),
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() {
}
