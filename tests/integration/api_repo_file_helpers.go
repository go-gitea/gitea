// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"strings"

	repo_model "code.gitea.io/gitea/internal/models/repo"
	user_model "code.gitea.io/gitea/internal/models/user"
	"code.gitea.io/gitea/internal/modules/git"
	api "code.gitea.io/gitea/internal/modules/structs"
	files_service "code.gitea.io/gitea/internal/services/repository/files"
)

func createFileInBranch(user *user_model.User, repo *repo_model.Repository, treePath, branchName, content string) (*api.FilesResponse, error) {
	opts := &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     "create",
				TreePath:      treePath,
				ContentReader: strings.NewReader(content),
			},
		},
		OldBranch: branchName,
		Author:    nil,
		Committer: nil,
	}
	return files_service.ChangeRepoFiles(git.DefaultContext, repo, user, opts)
}

func createFile(user *user_model.User, repo *repo_model.Repository, treePath string) (*api.FilesResponse, error) {
	return createFileInBranch(user, repo, treePath, repo.DefaultBranch, "This is a NEW file")
}
