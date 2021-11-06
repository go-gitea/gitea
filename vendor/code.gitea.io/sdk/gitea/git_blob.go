// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// GitBlobResponse represents a git blob
type GitBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	URL      string `json:"url"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
}

// GetBlob get the blob of a repository file
func (c *Client) GetBlob(user, repo, sha string) (*GitBlobResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &sha); err != nil {
		return nil, nil, err
	}
	blob := new(GitBlobResponse)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/git/blobs/%s", user, repo, sha), nil, nil, blob)
	return blob, resp, err
}
