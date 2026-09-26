// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"os"
	"path/filepath"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"

	commitgraph "github.com/go-git/go-git/v5/plumbing/format/commitgraph/v2"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// CommitNodeIndex returns the index for walking commit graph
func (repo *Repository) CommitNodeIndex() (_ cgobject.CommitNodeIndex, closer func()) {
	indexPath := filepath.Join(gitrepo.RepoLocalPath(repo), "objects", "info", "commit-graph")
	file, err := os.Open(indexPath)
	if err == nil {
		var index commitgraph.Index
		index, err = commitgraph.OpenFileIndex(file)
		if err == nil {
			return cgobject.NewGraphCommitNodeIndex(index, repo.gogitRepo.Storer), func() { _ = file.Close() }
		}
		_ = file.Close()
	}

	if !os.IsNotExist(err) {
		log.Warn("Unable to read commit-graph for %s: %v", repo.LogString(), err)
	}

	return cgobject.NewObjectCommitNodeIndex(repo.gogitRepo.Storer), func() {}
}
