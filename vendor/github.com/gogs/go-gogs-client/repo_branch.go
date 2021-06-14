// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"fmt"
)

// Branch represents a repository branch.
type Branch struct {
	Name   string         `json:"name"`
	Commit *PayloadCommit `json:"commit"`
}

func (c *Client) ListRepoBranches(user, repo string) ([]*Branch, error) {
	branches := make([]*Branch, 0, 10)
	return branches, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches", user, repo), nil, nil, &branches)
}

func (c *Client) GetRepoBranch(user, repo, branch string) (*Branch, error) {
	b := new(Branch)
	return b, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/branches/%s", user, repo, branch), nil, nil, &b)
}
