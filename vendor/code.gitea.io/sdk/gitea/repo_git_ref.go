// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"strings"
)

// GitObject represents a repository git object
type GitObject struct {
	Type string `json:"type"`
	Sha  string `json:"sha"`
	URL  string `json:"url"`
}

// GitReference represents a repository git reference
type GitReference struct {
	Ref    string     `json:"ref"`
	URL    string     `json:"url"`
	Object *GitObject `json:"object"`
}

// GetGitRefs list all the refs starting with the provided ref of one repository
func (c *Client) GetGitRefs(user, repo, ref string) ([]*GitReference, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	buf, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil)
	if err != nil {
		return nil, err
	}
}

// GetGitRef get one ref's information of one repository
func (c *Client) GetGitRef(user, repo, ref string) (*GitReference, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	buf, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil)
	if err != nil {
		return nil, err
	}
}
