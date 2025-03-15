// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "context"

// GetBlobByPath get the blob object according the path
func (t *Tree) GetBlobByPath(ctx context.Context, relpath string) (*Blob, error) {
	entry, err := t.GetTreeEntryByPath(ctx, relpath)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir() && !entry.IsSubModule() {
		return entry.Blob(), nil
	}

	return nil, ErrNotExist{"", relpath}
}
