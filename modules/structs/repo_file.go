// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// FileOptions options for all file APIs
type FileOptions struct {
	// message (optional) for the commit of this file. if not supplied, a default message will be used
	Message string `json:"message"`
	// branch (optional) to base this file from. if not given, the default branch is used
	BranchName string `json:"branch" binding:"GitRefName;MaxSize(100)"`
	// new_branch (optional) will make a new branch from `branch` before creating the file
	NewBranchName string `json:"new_branch" binding:"GitRefName;MaxSize(100)"`
	// `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
	Author    Identity          `json:"author"`
	Committer Identity          `json:"committer"`
	Dates     CommitDateOptions `json:"dates"`
	// Add a Signed-off-by trailer by the committer at the end of the commit log message.
	Signoff bool `json:"signoff"`
}

type FileOptionsWithSHA struct {
	FileOptions
	// the blob ID (SHA) for the file that already exists, it is required for changing existing files
	// required: true
	SHA string `json:"sha" binding:"Required"`
}

func (f *FileOptions) GetFileOptions() *FileOptions {
	return f
}

type FileOptionsInterface interface {
	GetFileOptions() *FileOptions
}

var _ FileOptionsInterface = (*FileOptions)(nil)

// CreateFileOptions options for creating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type CreateFileOptions struct {
	FileOptions
	// content must be base64 encoded
	// required: true
	ContentBase64 string `json:"content"`
}

// DeleteFileOptions options for deleting files (used for other File structs below)
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type DeleteFileOptions struct {
	FileOptionsWithSHA
}

// UpdateFileOptions options for updating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type UpdateFileOptions struct {
	FileOptionsWithSHA
	// content must be base64 encoded
	// required: true
	ContentBase64 string `json:"content"`
	// from_path (optional) is the path of the original file which will be moved/renamed to the path in the URL
	FromPath string `json:"from_path" binding:"MaxSize(500)"`
}

// FIXME: there is no LastCommitID in FileOptions, actually it should be an alternative to the SHA in ChangeFileOperation

// ChangeFileOperation for creating, updating or deleting a file
type ChangeFileOperation struct {
	// indicates what to do with the file: "create" for creating a new file, "update" for updating an existing file,
	// "upload" for creating or updating a file, "rename" for renaming a file, and "delete" for deleting an existing file.
	// required: true
	// enum: create,update,upload,rename,delete
	Operation string `json:"operation" binding:"Required"`
	// path to the existing or new file
	// required: true
	Path string `json:"path" binding:"Required;MaxSize(500)"`
	// new or updated file content, it must be base64 encoded
	ContentBase64 string `json:"content"`
	// the blob ID (SHA) for the file that already exists, required for changing existing files
	SHA string `json:"sha"`
	// old path of the file to move
	FromPath string `json:"from_path"`
}

// ChangeFilesOptions options for creating, updating or deleting multiple files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type ChangeFilesOptions struct {
	FileOptions
	// list of file operations
	// required: true
	Files []*ChangeFileOperation `json:"files" binding:"Required"`
}

// ApplyDiffPatchFileOptions options for applying a diff patch
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type ApplyDiffPatchFileOptions struct {
	FileOptions
	// required: true
	Content string `json:"content"`
}

// FileLinksResponse contains the links for a repo's file
type FileLinksResponse struct {
	// Self is the API URL for this file
	Self *string `json:"self"`
	// GitURL is the Git API URL for this file
	GitURL *string `json:"git"`
	// HTMLURL is the web URL for this file
	HTMLURL *string `json:"html"`
}

type ContentsExtResponse struct {
	// FileContents contains file information when the path represents a file
	FileContents *ContentsResponse `json:"file_contents,omitempty"`
	// DirContents contains directory listing when the path represents a directory
	DirContents []*ContentsResponse `json:"dir_contents,omitempty"`
}

// ContentsResponse contains information about a repo's entry's (dir, file, symlink, submodule) metadata and content
type ContentsResponse struct {
	// Name is the file or directory name
	Name string `json:"name"`
	// Path is the full path to the file or directory
	Path string `json:"path"`
	// SHA is the Git blob or tree SHA
	SHA string `json:"sha"`

	// LastCommitSHA is the SHA of the last commit that affected this file
	LastCommitSHA *string `json:"last_commit_sha,omitempty"`
	// swagger:strfmt date-time
	LastCommitterDate *time.Time `json:"last_committer_date,omitempty"`
	// swagger:strfmt date-time
	LastAuthorDate *time.Time `json:"last_author_date,omitempty"`
	// LastCommitMessage is the message of the last commit that affected this file
	LastCommitMessage *string `json:"last_commit_message,omitempty"`

	// `type` will be `file`, `dir`, `symlink`, or `submodule`
	Type string `json:"type"`
	// Size is the file size in bytes
	Size int64 `json:"size"`
	// `encoding` is populated when `type` is `file`, otherwise null
	Encoding *string `json:"encoding"`
	// `content` is populated when `type` is `file`, otherwise null
	Content *string `json:"content"`
	// `target` is populated when `type` is `symlink`, otherwise null
	Target *string `json:"target"`
	// URL is the API URL for this file or directory
	URL *string `json:"url"`
	// HTMLURL is the web URL for this file or directory
	HTMLURL *string `json:"html_url"`
	// GitURL is the Git API URL for this blob or tree
	GitURL *string `json:"git_url"`
	// DownloadURL is the direct download URL for this file
	DownloadURL *string `json:"download_url"`
	// `submodule_git_url` is populated when `type` is `submodule`, otherwise null
	SubmoduleGitURL *string `json:"submodule_git_url"`
	// Links contains related URLs for this file or directory
	Links *FileLinksResponse `json:"_links"`

	// LfsOid is the Git LFS object ID if this file is stored in LFS
	LfsOid *string `json:"lfs_oid,omitempty"`
	// LfsSize is the file size if this file is stored in LFS
	LfsSize *int64 `json:"lfs_size,omitempty"`
}

// FileCommitResponse contains information generated from a Git commit for a repo's file.
type FileCommitResponse struct {
	CommitMeta
	// HTMLURL is the web URL for viewing this commit
	HTMLURL string `json:"html_url"`
	// Author is the commit author information
	Author *CommitUser `json:"author"`
	// Committer is the commit committer information
	Committer *CommitUser `json:"committer"`
	// Parents contains parent commit metadata
	Parents []*CommitMeta `json:"parents"`
	// Message is the commit message
	Message string `json:"message"`
	// Tree contains the tree metadata for this commit
	Tree *CommitMeta `json:"tree"`
}

// FileResponse contains information about a repo's file
type FileResponse struct {
	// Content contains the file content and metadata
	Content *ContentsResponse `json:"content"`
	// Commit contains the commit information for this file operation
	Commit *FileCommitResponse `json:"commit"`
	// Verification contains the commit signature verification information
	Verification *PayloadCommitVerification `json:"verification"`
}

// FilesResponse contains information about multiple files from a repo
type FilesResponse struct {
	// Files contains the list of file contents and metadata
	Files []*ContentsResponse `json:"files"`
	// Commit contains the commit information for this file operation
	Commit *FileCommitResponse `json:"commit"`
	// Verification contains the commit signature verification information
	Verification *PayloadCommitVerification `json:"verification"`
}

// FileDeleteResponse contains information about a repo's file that was deleted
type FileDeleteResponse struct {
	// Content is always null for delete operations
	Content any `json:"content"` // to be set to nil
	// Commit contains the commit information for this delete operation
	Commit *FileCommitResponse `json:"commit"`
	// Verification contains the commit signature verification information
	Verification *PayloadCommitVerification `json:"verification"`
}

// GetFilesOptions options for retrieving metadate and content of multiple files
type GetFilesOptions struct {
	// Files is the list of file paths to retrieve
	Files []string `json:"files" binding:"Required"`
}
