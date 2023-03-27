// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

func absPath(path string) string {
	return filepath.Join(setting.RepoRootPath, path)
}

// UserRelPath returns the path relative path of user repositories.
func UserRelPath(userName string) string {
	return strings.ToLower(userName)
}

// UserPath returns the path absolute path of user repositories.
func UserPath(userName string) string { //revive:disable-line:exported
	return absPath(strings.ToLower(userName))
}

// RepoRelPath returns repository relative path by given user and repository name.
func RepoRelPath(userName, repoName string) string {
	return path.Join(strings.ToLower(userName), strings.ToLower(repoName)+".git")
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string { //revive:disable-line:exported
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".git")
}

// WikiRelPath returns wiki repository relative path by given user and repository name.
func WikiRelPath(userName, repoName string) string {
	return path.Join(strings.ToLower(userName), strings.ToLower(repoName)+".wiki.git")
}

// WikiPath returns wiki data path by given user and repository name.
func WikiPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName))
}
