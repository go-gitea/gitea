// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"strings"

	"code.gitea.io/gitea/models"
)

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func GetDiffPreview(repo *models.Repository, branch, treePath, content string) (*models.Diff, error) {
	t, err := NewTemporaryUploadRepository(repo)
	defer t.Close()
	if err != nil {
		return nil, err
	}
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
	return t.DiffIndex()
}
