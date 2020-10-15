// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// Tag represents a repository tag
type Tag struct {
	Name       string      `json:"name"`
	ID         string      `json:"id"`
	Commit     *CommitMeta `json:"commit"`
	ZipballURL string      `json:"zipball_url"`
	TarballURL string      `json:"tarball_url"`
}

// ListRepoTagsOptions options for listing a repository's tags
type ListRepoTagsOptions struct {
	ListOptions
}

// ListRepoTags list all the branches of one repository
func (c *Client) ListRepoTags(user, repo string, opt ListRepoTagsOptions) ([]*Tag, *Response, error) {
	opt.setDefaults()
	tags := make([]*Tag, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/tags?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &tags)
	return tags, resp, err
}
