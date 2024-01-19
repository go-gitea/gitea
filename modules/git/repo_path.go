// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "code.gitea.io/gitea/modules/util"

func RenameRepo(oldDir, newDir string) error {
	return util.Rename(oldDir, newDir)
}
