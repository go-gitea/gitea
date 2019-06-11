// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"fmt"
)

// GetFile downloads a file of repository, ref can be branch/tag/commit.
// e.g.: ref -> master, tree -> macaron.go(no leading slash)
func (c *Client) GetFile(user, repo, ref, tree string) ([]byte, error) {
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/raw/%s/%s", user, repo, ref, tree), nil, nil)
}

// GetArchive downloads the full contents of a repository. Ref can be a branch/tag/commit.
func (c *Client) GetArchive(user, repo, ref, format string) ([]byte, error) {
	if format != ".zip" && format != ".tar.gz" {
		return nil, fmt.Errorf("invalid format: %s (must be .zip or .tar.gz)", format)
	}
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/archive/%s%s", user, repo, ref, format), nil, nil)
}
