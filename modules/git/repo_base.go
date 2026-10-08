// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

type Repository struct {
	RepositoryBase
}

func openRepositoryInternal(_ *Repository) error {
	return nil
}

func (repo *Repository) closeInternal() error {
	return nil
}
