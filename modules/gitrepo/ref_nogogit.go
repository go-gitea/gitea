// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"context"
	"strings"
)

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func GetRefsBySha(ctx context.Context, repo Repository, sha, prefix string) ([]string, error) {
	var revList []string
	_, err := WalkShowRef(ctx, repo, nil, 0, 0, func(walkSha, refname string) error {
		if walkSha == sha && strings.HasPrefix(refname, prefix) {
			revList = append(revList, refname)
		}
		return nil
	})
	return revList, err
}
