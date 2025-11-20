// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/stretchr/testify/require"
)

type createFileInBranchOptions struct {
	OldBranch, NewBranch string
	CommitMessage        string
	CommitterName        string
	CommitterEmail       string
}

func testCreateFileInBranch(t *testing.T, user *user_model.User, repo *repo_model.Repository, createOpts createFileInBranchOptions, files map[string]string) *api.FilesResponse {
	resp, err := createFileInBranch(user, repo, createOpts, files)
	require.NoError(t, err)
	return resp
}

func createFileInBranch(user *user_model.User, repo *repo_model.Repository, createOpts createFileInBranchOptions, files map[string]string) (*api.FilesResponse, error) {
	ctx := context.TODO()
	opts := &files_service.ChangeRepoFilesOptions{
		OldBranch: createOpts.OldBranch,
		NewBranch: createOpts.NewBranch,
		Message:   createOpts.CommitMessage,
	}
	if createOpts.CommitterName != "" || createOpts.CommitterEmail != "" {
		opts.Committer = &files_service.IdentityOptions{
			GitUserName:  createOpts.CommitterName,
			GitUserEmail: createOpts.CommitterEmail,
		}
	}
	for path, content := range files {
		opts.Files = append(opts.Files, &files_service.ChangeRepoFile{
			Operation:     "create",
			TreePath:      path,
			ContentReader: strings.NewReader(content),
		})
	}
	return files_service.ChangeRepoFiles(ctx, repo, user, opts)
}

func deleteFileInBranch(user *user_model.User, repo *repo_model.Repository, treePath, branchName string) (*api.FilesResponse, error) {
	ctx := context.TODO()
	opts := &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "delete",
				TreePath:  treePath,
			},
		},
		OldBranch: branchName,
		Author:    nil,
		Committer: nil,
	}
	return files_service.ChangeRepoFiles(ctx, repo, user, opts)
}

func createOrReplaceFileInBranch(user *user_model.User, repo *repo_model.Repository, treePath, branchName, content string) error {
	_, err := deleteFileInBranch(user, repo, treePath, branchName)

	if err != nil && !files_service.IsErrRepoFileDoesNotExist(err) {
		return err
	}

	_, err = createFileInBranch(user, repo, createFileInBranchOptions{OldBranch: branchName}, map[string]string{treePath: content})
	return err
}

// TODO: replace all usages of this function with testCreateFileInBranch or testCreateFile
func createFile(user *user_model.User, repo *repo_model.Repository, treePath string, optContent ...string) (*api.FilesResponse, error) {
	content := util.OptionalArg(optContent, "This is a NEW file") // some tests need this default content because its SHA is hardcoded
	return createFileInBranch(user, repo, createFileInBranchOptions{}, map[string]string{treePath: content})
}
