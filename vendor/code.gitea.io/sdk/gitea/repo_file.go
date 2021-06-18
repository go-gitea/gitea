// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// FileOptions options for all file APIs
type FileOptions struct {
	// message (optional) for the commit of this file. if not supplied, a default message will be used
	Message string `json:"message"`
	// branch (optional) to base this file from. if not given, the default branch is used
	BranchName string `json:"branch"`
	// new_branch (optional) will make a new branch from `branch` before creating the file
	NewBranchName string `json:"new_branch"`
	// `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
	Author    Identity          `json:"author"`
	Committer Identity          `json:"committer"`
	Dates     CommitDateOptions `json:"dates"`
	// Add a Signed-off-by trailer by the committer at the end of the commit log message.
	Signoff bool `json:"signoff"`
}

// CreateFileOptions options for creating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type CreateFileOptions struct {
	FileOptions
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
}

// DeleteFileOptions options for deleting files (used for other File structs below)
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type DeleteFileOptions struct {
	FileOptions
	// sha is the SHA for the file that already exists
	// required: true
	SHA string `json:"sha"`
}

// UpdateFileOptions options for updating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type UpdateFileOptions struct {
	FileOptions
	// sha is the SHA for the file that already exists
	// required: true
	SHA string `json:"sha"`
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
	// from_path (optional) is the path of the original file which will be moved/renamed to the path in the URL
	FromPath string `json:"from_path"`
}

// FileLinksResponse contains the links for a repo's file
type FileLinksResponse struct {
	Self    *string `json:"self"`
	GitURL  *string `json:"git"`
	HTMLURL *string `json:"html"`
}

// ContentsResponse contains information about a repo's entry's (dir, file, symlink, submodule) metadata and content
type ContentsResponse struct {
	Name string `json:"name"`
	Path string `json:"path"`
	SHA  string `json:"sha"`
	// `type` will be `file`, `dir`, `symlink`, or `submodule`
	Type string `json:"type"`
	Size int64  `json:"size"`
	// `encoding` is populated when `type` is `file`, otherwise null
	Encoding *string `json:"encoding"`
	// `content` is populated when `type` is `file`, otherwise null
	Content *string `json:"content"`
	// `target` is populated when `type` is `symlink`, otherwise null
	Target      *string `json:"target"`
	URL         *string `json:"url"`
	HTMLURL     *string `json:"html_url"`
	GitURL      *string `json:"git_url"`
	DownloadURL *string `json:"download_url"`
	// `submodule_git_url` is populated when `type` is `submodule`, otherwise null
	SubmoduleGitURL *string            `json:"submodule_git_url"`
	Links           *FileLinksResponse `json:"_links"`
}

// FileCommitResponse contains information generated from a Git commit for a repo's file.
type FileCommitResponse struct {
	CommitMeta
	HTMLURL   string        `json:"html_url"`
	Author    *CommitUser   `json:"author"`
	Committer *CommitUser   `json:"committer"`
	Parents   []*CommitMeta `json:"parents"`
	Message   string        `json:"message"`
	Tree      *CommitMeta   `json:"tree"`
}

// FileResponse contains information about a repo's file
type FileResponse struct {
	Content      *ContentsResponse          `json:"content"`
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// FileDeleteResponse contains information about a repo's file that was deleted
type FileDeleteResponse struct {
	Content      interface{}                `json:"content"` // to be set to nil
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// GetFile downloads a file of repository, ref can be branch/tag/commit.
// e.g.: ref -> master, filepath -> README.md (no leading slash)
func (c *Client) GetFile(owner, repo, ref, filepath string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(filepath)
	if c.checkServerVersionGreaterThanOrEqual(version1_14_0) != nil {
		ref = pathEscapeSegments(ref)
		return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/raw/%s/%s", owner, repo, ref, filepath), nil, nil)
	}
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/raw/%s?ref=%s", owner, repo, filepath, url.QueryEscape(ref)), nil, nil)
}

// GetContents get the metadata and contents of a file in a repository
// ref is optional
func (c *Client) GetContents(owner, repo, ref, filepath string) (*ContentsResponse, *Response, error) {
	data, resp, err := c.getDirOrFileContents(owner, repo, ref, filepath)
	if err != nil {
		return nil, resp, err
	}
	cr := new(ContentsResponse)
	if json.Unmarshal(data, &cr) != nil {
		return nil, resp, fmt.Errorf("expect file, got directory")
	}
	return cr, resp, err
}

// ListContents gets a list of entries in a dir
// ref is optional
func (c *Client) ListContents(owner, repo, ref, filepath string) ([]*ContentsResponse, *Response, error) {
	data, resp, err := c.getDirOrFileContents(owner, repo, ref, filepath)
	if err != nil {
		return nil, resp, err
	}
	crl := make([]*ContentsResponse, 0)
	if json.Unmarshal(data, &crl) != nil {
		return nil, resp, fmt.Errorf("expect directory, got file")
	}
	return crl, resp, err
}

func (c *Client) getDirOrFileContents(owner, repo, ref, filepath string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(strings.TrimPrefix(filepath, "/"))
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, filepath, url.QueryEscape(ref)), jsonHeader, nil)
}

// CreateFile create a file in a repository
func (c *Client) CreateFile(owner, repo, filepath string, opt CreateFileOptions) (*FileResponse, *Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(owner, repo, opt.BranchName); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	fr := new(FileResponse)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body), fr)
	return fr, resp, err
}

// UpdateFile update a file in a repository
func (c *Client) UpdateFile(owner, repo, filepath string, opt UpdateFileOptions) (*FileResponse, *Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(owner, repo, opt.BranchName); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	fr := new(FileResponse)
	resp, err := c.getParsedResponse("PUT", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body), fr)
	return fr, resp, err
}

// DeleteFile delete a file from repository
func (c *Client) DeleteFile(owner, repo, filepath string, opt DeleteFileOptions) (*Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(owner, repo, opt.BranchName); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != 200 && status != 204 {
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
	return resp, nil
}

func (c *Client) setDefaultBranchForOldVersions(owner, repo, branch string) (string, error) {
	if len(branch) == 0 {
		// Gitea >= 1.12.0 Use DefaultBranch on "", mimic this for older versions
		if c.checkServerVersionGreaterThanOrEqual(version1_12_0) != nil {
			r, _, err := c.GetRepo(owner, repo)
			if err != nil {
				return "", err
			}
			return r.DefaultBranch, nil
		}
	}
	return branch, nil
}
