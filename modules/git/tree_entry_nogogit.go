// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"

	"gitea.dev/modules/log"
)

func (te *TreeEntry) GetSize(ctx context.Context, gitRepo *Repository) int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	batch, cancel, err := gitRepo.CatFileBatch(ctx)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), gitRepo.Path, err)
		return 0
	}
	defer cancel()
	info, err := batch.QueryInfo(te.ID.String())
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), gitRepo.Path, err)
		return 0
	}

	te.size = info.Size
	te.sized = true
	return te.size
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob(gitRepo *Repository) *Blob {
	return &Blob{
		ID:      te.ID,
		name:    te.Name(),
		size:    te.size,
		gotSize: te.sized,
		repo:    gitRepo,
	}
}
