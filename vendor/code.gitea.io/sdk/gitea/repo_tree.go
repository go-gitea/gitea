// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// GitEntry represents a git tree
type GitEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitTreeResponse returns a git tree
type GitTreeResponse struct {
	SHA        string     `json:"sha"`
	URL        string     `json:"url"`
	Entries    []GitEntry `json:"tree"`
	Truncated  bool       `json:"truncated"`
	Page       int        `json:"page"`
	TotalCount int        `json:"total_count"`
}

// GetTrees downloads a file of repository, ref can be branch/tag/commit.
// e.g.: ref -> master, tree -> macaron.go(no leading slash)
func (c *Client) GetTrees(user, repo, ref string, recursive bool) (*GitTreeResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &ref); err != nil {
		return nil, nil, err
	}
	trees := new(GitTreeResponse)
	var path = fmt.Sprintf("/repos/%s/%s/git/trees/%s", user, repo, ref)
	if recursive {
		path += "?recursive=1"
	}
	resp, err := c.getParsedResponse("GET", path, nil, nil, trees)
	return trees, resp, err
}
