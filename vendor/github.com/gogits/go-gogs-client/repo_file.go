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
