// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// GetFile downloads a file of repository, ref can be branch/tag/commit.
// e.g.: ref -> master, tree -> macaron.go(no leading slash)
func (c *Client) GetFile(user, repo, ref, tree string) ([]byte, error) {
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/raw/%s/%s", user, repo, ref, tree), nil, nil)
}

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	// required: true
	Name  string `json:"name" binding:"MaxSize(100)"`
	// required: true
	// swagger:strfmt email
	Email string `json:"email" binding:"Required;Email;MaxSize(254)"`
}

// CreateFileOptions options to create or update a file in a repo
type CreateFileOptions struct {
	Message     string `json:"message"" binding:"Required"`
	Content     string `json:"content"`
	Branch      string `json:"branch"`
	Author 	    User   `json:"author"`
	Committer   User   `json:"committer"`
}

// UpdateFileOptions options to create or update a file in a repo
type UpdateFileOptions struct {
	CreateFileOptions
	SHA	    string `json:"sha" binding:"Required"`
}

// DeleteFileOptions options to create or update a file in a repo
type DeleteFileOptions struct {
	UpdateFileOptions
}

// FileLink contains the links for a repo's file
type FileLink struct {
	SelfL    string `json:"url"`
	GitURL    string `json:"git_url"`
	HTMLURL    string `json:"html_url"`
}

// FileContent contains information about a repo's file stats and content
type FileContent struct {
	name   string        `json:"name"`
	path   string         `json:"path"`
	SHA    string `json:"sha"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
	HTMLURL    string `json:"html_url"`
	GitURL    string `json:"git_url"`
	DownloadURL    string `json:"download_url"`
	Type      string        `json:"type"`
	Links   []*FileLink        `json:"_links"`
}

// FileCommit contains information generated from a Git commit for a repo's file.
type FileCommit struct {
	*CommitMeta
	HTMLURL   string        `json:"html_url"`
	Author    *User         `json:"author"`
	Committer *User         `json:"committer"`
	Parents   []*CommitMeta `json:"parents"`
	NodeID    string        `json:"node_id"`
	Message   string        `json:"message"`
	Tree      *CommitMeta `json:"tree"`
}

// FileResponse contains information about a repo's file
type FileResponpse struct {
	Content	     FileContent               `json:"content"`
	Commit       FileCommit                   `json:"commit"`
	Verification PayloadCommitVerification `json:"verification"`
}


