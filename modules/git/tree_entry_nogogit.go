// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import "code.gitea.io/gitea/modules/log"

// Size returns the size of the entry
func (te *TreeEntry) Size() int64 {
	if te.IsDir() {
		return 0
	} else if te.sized {
		return te.size
	}

	batch, cancel, err := te.ptree.repo.CatFileBatchCheck(te.ptree.repo.Ctx)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), te.ptree.repo.Path, err)
		return 0
	}
	defer cancel()
	_, err = batch.Writer().Write([]byte(te.ID.String() + "\n"))
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), te.ptree.repo.Path, err)
		return 0
	}
	_, _, te.size, err = ReadBatchLine(batch.Reader())
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", te.ID.String(), te.ptree.repo.Path, err)
		return 0
	}

	te.sized = true
	return te.size
}

// Blob returns the blob object the entry
func (te *TreeEntry) Blob() *Blob {
	return &Blob{
		ID:      te.ID,
		name:    te.Name(),
		size:    te.size,
		gotSize: te.sized,
		repo:    te.ptree.repo,
	}
}
