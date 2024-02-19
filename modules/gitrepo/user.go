// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func RenameDir(ctx context.Context, oldName, newName string) error {
	return util.Rename(filepath.Join(setting.RepoRootPath, strings.ToLower(oldName)), filepath.Join(setting.RepoRootPath, strings.ToLower(newName)))
}
