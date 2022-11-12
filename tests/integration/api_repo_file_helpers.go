// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	files_service "code.gitea.io/gitea/services/repository/files"
)

func createFileInBranch(user *user_model.User, repo *repo_model.Repository, treePath, branchName, content string) (*api.FileResponse, error) {
	opts := &files_service.UpdateRepoFileOptions{
		OldBranch: branchName,
		TreePath:  treePath,
		Content:   content,
		IsNewFile: true,
		Author:    nil,
		Committer: nil,
	}
	return files_service.CreateOrUpdateRepoFile(git.DefaultContext, repo, user, opts)
}

func createFile(user *user_model.User, repo *repo_model.Repository, treePath string) (*api.FileResponse, error) {
	return createFileInBranch(user, repo, treePath, repo.DefaultBranch, "This is a NEW file")
}
