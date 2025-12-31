// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package gitrepo

import (
	"context"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func GetRefsBySha(ctx context.Context, repo Repository, sha, prefix string) ([]string, error) {
	var revList []string
	gitRepo, closer, err := RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	iter, err := gitRepo.GogitRepo().References()
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Hash().String() == sha && strings.HasPrefix(string(ref.Name()), prefix) {
			revList = append(revList, string(ref.Name()))
		}
		return nil
	})
	return revList, err
}
