// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/gitdiff"
)

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func GetDiffPreview(ctx context.Context, repo *repo_model.Repository, branch, treePath, content string) (*gitdiff.Diff, error) {
	if branch == "" {
		branch = repo.DefaultBranch
	}
	t, err := NewTemporaryUploadRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.Clone(branch); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}

	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex("100644", objectHash, treePath); err != nil {
		return nil, err
	}
	diff, err := t.DiffIndex()
	if err != nil {
		return nil, err
	}

	if len(diff.Files) != 1 {
		return diff, nil
	}
	if diff.Files[0].Type == gitdiff.DiffFileAdd {
		diff.Files[0].FullFileHiglight(nil, nil, nil, []byte(content))
		return diff, nil
	}

	commitID, err := t.GetLastCommit()
	if err != nil {
		log.Error("GetLastCommit: %v", err)
		return diff, nil
	}

	commit, err := t.GetCommit(commitID)
	if err != nil {
		log.Error("GetCommit: %v", err)
		return diff, nil
	}

	diff.Files[0].FullFileHiglight(commit, nil, nil, []byte(content))

	return diff, nil
}
