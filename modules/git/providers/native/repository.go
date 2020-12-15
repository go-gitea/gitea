// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Repository) = &Repository{}

// Repository represents a git repository
type Repository struct {
	path string

	gpgSettings *service.GPGSettings
}

// Close implements io.Closer
func (repo *Repository) Close() error {
	return nil
}

// Path is the filesystem path for the repository
func (repo *Repository) Path() string {
	return repo.path
}

//  _
// |_) |  _  |_
// |_) | (_) |_)
//

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (service.Blob, error) {
	id, err := StringHash("").FromString(idStr)
	if err != nil {
		return nil, err
	}

	return &Blob{Object: &Object{hash: id, repo: repo}}, nil
}

//  __  _   __
// /__ |_) /__
// \_| |   \_|
//

// GetDefaultPublicGPGKey will return and cache the default public GPG settings for this repository
func (repo *Repository) GetDefaultPublicGPGKey(forceUpdate bool) (*service.GPGSettings, error) {
	if repo.gpgSettings != nil && !forceUpdate {
		return repo.gpgSettings, nil
	}

	var err error
	repo.gpgSettings, err = common.GetDefaultPublicGPGKey(repo.Path())
	return repo.gpgSettings, err
}

//  _
// |_) |  _. ._ _   _
// |_) | (_| | | | (/_
//

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, path, file string, line uint) (service.Commit, error) {
	res, err := git.NewCommand("blame", fmt.Sprintf("-L %d,%d", line, line), "-p", revision, "--", file).RunInDir(path)
	if err != nil {
		return nil, err
	}
	if len(res) < 40 {
		return nil, fmt.Errorf("invalid result of blame: %s", res)
	}
	return repo.GetCommit(res[:40])
}

//  __
// (_   _  ._    o  _  _
// 	__) (/_ |  \/ | (_ (/_
//

// Service returns this repositories preferred service
func (repo *Repository) Service() service.GitService {
	return gitService
}
