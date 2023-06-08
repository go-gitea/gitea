// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

func absPath(path string) string {
	return filepath.Join(setting.RepoRootPath, path)
}

// UserPath returns the path absolute path of user repositories.
func UserPath(userName string) string { //revive:disable-line:exported
	return absPath(strings.ToLower(userName))
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string { //revive:disable-line:exported
	return absPath(filepath.Join(strings.ToLower(userName), strings.ToLower(repoName)+".git"))
}

// LocalPath returns local path by given relative path.
func LocalPath(relPath string) string {
	return absPath(relPath)
}

// WikiPath returns wiki data path by given user and repository name.
func WikiPath(userName, repoName string) string {
	return absPath(filepath.Join(strings.ToLower(userName), strings.ToLower(repoName)+".wiki.git"))
}
