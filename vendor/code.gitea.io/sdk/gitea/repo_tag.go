// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// Tag represents a repository tag
type Tag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
}

// ListRepoTags list all the branches of one repository
func (c *Client) ListRepoTags(user, repo string) ([]*Tag, error) {
	tags := make([]*Tag, 0, 10)
	return tags, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/tags", user, repo), nil, nil, &tags)
}
