// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

// TreeGetBlobByPath is a common method to get the blob object according the path
func TreeGetBlobByPath(t service.Tree, relpath string) (service.Blob, error) {
	entry, err := t.GetTreeEntryByPath(relpath)
	if err != nil {
		return nil, err
	}

	if !entry.Mode().IsDir() && !entry.Mode().IsSubModule() {
		return entry.(service.Blob), nil
	}

	return nil, git.ErrNotExist{ID: "", RelPath: relpath}
}
