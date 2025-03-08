// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/services/gitdiff"
)

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func GetDiffPreview(ctx context.Context, repo *repo_model.Repository, branch, treePath, content string) (*gitdiff.Diff, error) {
	if branch == "" {
		branch = repo.DefaultBranch
	}
	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.Clone(ctx, branch, true); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(ctx); err != nil {
		return nil, err
	}

	// Add the object to the database
	objectHash, err := t.HashObject(ctx, strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex(ctx, "100644", objectHash, treePath); err != nil {
		return nil, err
	}
	return t.DiffIndex(ctx)
}
