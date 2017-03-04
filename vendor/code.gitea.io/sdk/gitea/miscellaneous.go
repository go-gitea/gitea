// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// MarkdownOption markdown options
type MarkdownOption struct {
	Text    string
	Mode    string
	Context string
	Wiki    bool
}

// ServerVersion wraps the version of the server
type ServerVersion struct {
	Version string
}

// ServerVersion returns the version of the server
func (c *Client) ServerVersion() (string, error) {
	v := ServerVersion{}
	return v.Version, c.getParsedResponse("GET", "/api/v1/version", nil, nil, &v)
}
