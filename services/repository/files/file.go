// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

func GetFilesResponseFromCommit(ctx context.Context, repo *repo_model.Repository, commit *git.Commit, branch string, treeNames []string) (*api.FilesResponse, error) {
	files := []*api.ContentsResponse{}
	for _, file := range treeNames {
		fileContents, _ := GetContents(ctx, repo, file, branch, false) // ok if fails, then will be nil
		files = append(files, fileContents)
	}
	fileCommitResponse, _ := GetFileCommitResponse(repo, commit) // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(ctx, commit)
	filesResponse := &api.FilesResponse{
		Files:        files,
		Commit:       fileCommitResponse,
		Verification: verification,
	}
	return filesResponse, nil
}

// GetFileResponseFromCommit Constructs a FileResponse from a Commit object
func GetFileResponseFromCommit(ctx context.Context, repo *repo_model.Repository, commit *git.Commit, branch, treeName string) (*api.FileResponse, error) {
	fileContents, _ := GetContents(ctx, repo, treeName, branch, false) // ok if fails, then will be nil
	fileCommitResponse, _ := GetFileCommitResponse(repo, commit)       // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(ctx, commit)
	fileResponse := &api.FileResponse{
		Content:      fileContents,
		Commit:       fileCommitResponse,
		Verification: verification,
	}
	return fileResponse, nil
}

// constructs a FileResponse with the file at the index from FilesResponse
func GetFileResponseFromFilesResponse(filesResponse *api.FilesResponse, index int) *api.FileResponse {
	content := &api.ContentsResponse{}
	if len(filesResponse.Files) > index {
		content = filesResponse.Files[index]
	}
	fileResponse := &api.FileResponse{
		Content:      content,
		Commit:       filesResponse.Commit,
		Verification: filesResponse.Verification,
	}
	return fileResponse
}

// GetFileCommitResponse Constructs a FileCommitResponse from a Commit object
func GetFileCommitResponse(repo *repo_model.Repository, commit *git.Commit) (*api.FileCommitResponse, error) {
	if repo == nil {
		return nil, fmt.Errorf("repo cannot be nil")
	}
	if commit == nil {
		return nil, fmt.Errorf("commit cannot be nil")
	}
	commitURL, _ := url.Parse(repo.APIURL() + "/git/commits/" + url.PathEscape(commit.ID.String()))
	commitTreeURL, _ := url.Parse(repo.APIURL() + "/git/trees/" + url.PathEscape(commit.Tree.ID.String()))
	parents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i <= commit.ParentCount(); i++ {
		if parent, err := commit.Parent(i); err == nil && parent != nil {
			parentCommitURL, _ := url.Parse(repo.APIURL() + "/git/commits/" + url.PathEscape(parent.ID.String()))
			parents[i] = &api.CommitMeta{
				SHA: parent.ID.String(),
				URL: parentCommitURL.String(),
			}
		}
	}
	commitHTMLURL, _ := url.Parse(repo.HTMLURL() + "/commit/" + url.PathEscape(commit.ID.String()))
	fileCommit := &api.FileCommitResponse{
		CommitMeta: api.CommitMeta{
			SHA: commit.ID.String(),
			URL: commitURL.String(),
		},
		HTMLURL: commitHTMLURL.String(),
		Author: &api.CommitUser{
			Identity: api.Identity{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
			},
			Date: commit.Author.When.UTC().Format(time.RFC3339),
		},
		Committer: &api.CommitUser{
			Identity: api.Identity{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
			},
			Date: commit.Committer.When.UTC().Format(time.RFC3339),
		},
		Message: commit.Message(),
		Tree: &api.CommitMeta{
			URL: commitTreeURL.String(),
			SHA: commit.Tree.ID.String(),
		},
		Parents: parents,
	}
	return fileCommit, nil
}

// ErrFilenameInvalid represents a "FilenameInvalid" kind of error.
type ErrFilenameInvalid struct {
	Path string
}

// IsErrFilenameInvalid checks if an error is an ErrFilenameInvalid.
func IsErrFilenameInvalid(err error) bool {
	_, ok := err.(ErrFilenameInvalid)
	return ok
}

func (err ErrFilenameInvalid) Error() string {
	return fmt.Sprintf("path contains a malformed path component [path: %s]", err.Path)
}

func (err ErrFilenameInvalid) Unwrap() error {
	return util.ErrInvalidArgument
}

// CleanUploadFileName Trims a filename and returns empty string if it is a .git directory
func CleanUploadFileName(name string) string {
	// Rebase the filename
	name = util.PathJoinRel(name)
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}
