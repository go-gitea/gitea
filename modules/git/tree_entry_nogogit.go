// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import "code.gitea.io/gitea/modules/log"

// GetSize returns the size of the entry
func (te *TreeEntry) GetSize(repo *Repository) int64 {
	if te.IsDir() {
		return 0
	} else if te.Sized {
		return te.Size
	}

	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), repo.Path, err)
		return 0
	}
	defer cancel()
	info, err := batch.QueryInfo(te.ID.String())
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), repo.Path, err)
		return 0
	}

	te.Size = info.Size
	te.Sized = true
	return te.Size
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		ID:      te.ID,
		name:    te.Name,
		size:    te.Size,
		gotSize: te.Sized,
	}
}
