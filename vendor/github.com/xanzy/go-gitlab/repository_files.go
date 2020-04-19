//
// Copyright 2017, Sander van Harmelen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gitlab

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
)

// RepositoryFilesService handles communication with the repository files
// related methods of the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/repository_files.html
type RepositoryFilesService struct {
	client *Client
}

// File represents a GitLab repository file.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/repository_files.html
type File struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Size     int    `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	Ref      string `json:"ref"`
	BlobID   string `json:"blob_id"`
	CommitID string `json:"commit_id"`
	SHA256   string `json:"content_sha256"`
}

func (r File) String() string {
	return Stringify(r)
}

// GetFileOptions represents the available GetFile() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-file-from-repository
type GetFileOptions struct {
	Ref *string `url:"ref,omitempty" json:"ref,omitempty"`
}

// GetFile allows you to receive information about a file in repository like
// name, size, content. Note that file content is Base64 encoded.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-file-from-repository
func (s *RepositoryFilesService) GetFile(pid interface{}, fileName string, opt *GetFileOptions, options ...RequestOptionFunc) (*File, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	f := new(File)
	resp, err := s.client.Do(req, f)
	if err != nil {
		return nil, resp, err
	}

	return f, resp, err
}

// GetFileMetaDataOptions represents the available GetFileMetaData() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-file-from-repository
type GetFileMetaDataOptions struct {
	Ref *string `url:"ref,omitempty" json:"ref,omitempty"`
}

// GetFileMetaData allows you to receive meta information about a file in
// repository like name, size.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-file-from-repository
func (s *RepositoryFilesService) GetFileMetaData(pid interface{}, fileName string, opt *GetFileMetaDataOptions, options ...RequestOptionFunc) (*File, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("HEAD", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return nil, resp, err
	}

	f := &File{
		BlobID:   resp.Header.Get("X-Gitlab-Blob-Id"),
		CommitID: resp.Header.Get("X-Gitlab-Last-Commit-Id"),
		Encoding: resp.Header.Get("X-Gitlab-Encoding"),
		FileName: resp.Header.Get("X-Gitlab-File-Name"),
		FilePath: resp.Header.Get("X-Gitlab-File-Path"),
		Ref:      resp.Header.Get("X-Gitlab-Ref"),
		SHA256:   resp.Header.Get("X-Gitlab-Content-Sha256"),
	}

	if sizeString := resp.Header.Get("X-Gitlab-Size"); sizeString != "" {
		f.Size, err = strconv.Atoi(sizeString)
		if err != nil {
			return nil, resp, err
		}
	}

	return f, resp, err
}

// GetRawFileOptions represents the available GetRawFile() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-raw-file-from-repository
type GetRawFileOptions struct {
	Ref *string `url:"ref,omitempty" json:"ref,omitempty"`
}

// GetRawFile allows you to receive the raw file in repository.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#get-raw-file-from-repository
func (s *RepositoryFilesService) GetRawFile(pid interface{}, fileName string, opt *GetRawFileOptions, options ...RequestOptionFunc) ([]byte, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s/raw",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var f bytes.Buffer
	resp, err := s.client.Do(req, &f)
	if err != nil {
		return nil, resp, err
	}

	return f.Bytes(), resp, err
}

// FileInfo represents file details of a GitLab repository file.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/repository_files.html
type FileInfo struct {
	FilePath string `json:"file_path"`
	Branch   string `json:"branch"`
}

func (r FileInfo) String() string {
	return Stringify(r)
}

// CreateFileOptions represents the available CreateFile() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#create-new-file-in-repository
type CreateFileOptions struct {
	Branch        *string `url:"branch,omitempty" json:"branch,omitempty"`
	Encoding      *string `url:"encoding,omitempty" json:"encoding,omitempty"`
	AuthorEmail   *string `url:"author_email,omitempty" json:"author_email,omitempty"`
	AuthorName    *string `url:"author_name,omitempty" json:"author_name,omitempty"`
	Content       *string `url:"content,omitempty" json:"content,omitempty"`
	CommitMessage *string `url:"commit_message,omitempty" json:"commit_message,omitempty"`
}

// CreateFile creates a new file in a repository.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#create-new-file-in-repository
func (s *RepositoryFilesService) CreateFile(pid interface{}, fileName string, opt *CreateFileOptions, options ...RequestOptionFunc) (*FileInfo, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("POST", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	f := new(FileInfo)
	resp, err := s.client.Do(req, f)
	if err != nil {
		return nil, resp, err
	}

	return f, resp, err
}

// UpdateFileOptions represents the available UpdateFile() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#update-existing-file-in-repository
type UpdateFileOptions struct {
	Branch        *string `url:"branch,omitempty" json:"branch,omitempty"`
	Encoding      *string `url:"encoding,omitempty" json:"encoding,omitempty"`
	AuthorEmail   *string `url:"author_email,omitempty" json:"author_email,omitempty"`
	AuthorName    *string `url:"author_name,omitempty" json:"author_name,omitempty"`
	Content       *string `url:"content,omitempty" json:"content,omitempty"`
	CommitMessage *string `url:"commit_message,omitempty" json:"commit_message,omitempty"`
	LastCommitID  *string `url:"last_commit_id,omitempty" json:"last_commit_id,omitempty"`
}

// UpdateFile updates an existing file in a repository
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#update-existing-file-in-repository
func (s *RepositoryFilesService) UpdateFile(pid interface{}, fileName string, opt *UpdateFileOptions, options ...RequestOptionFunc) (*FileInfo, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	f := new(FileInfo)
	resp, err := s.client.Do(req, f)
	if err != nil {
		return nil, resp, err
	}

	return f, resp, err
}

// DeleteFileOptions represents the available DeleteFile() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#delete-existing-file-in-repository
type DeleteFileOptions struct {
	Branch        *string `url:"branch,omitempty" json:"branch,omitempty"`
	AuthorEmail   *string `url:"author_email,omitempty" json:"author_email,omitempty"`
	AuthorName    *string `url:"author_name,omitempty" json:"author_name,omitempty"`
	CommitMessage *string `url:"commit_message,omitempty" json:"commit_message,omitempty"`
}

// DeleteFile deletes an existing file in a repository
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/repository_files.html#delete-existing-file-in-repository
func (s *RepositoryFilesService) DeleteFile(pid interface{}, fileName string, opt *DeleteFileOptions, options ...RequestOptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf(
		"projects/%s/repository/files/%s",
		pathEscape(project),
		url.PathEscape(fileName),
	)

	req, err := s.client.NewRequest("DELETE", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
