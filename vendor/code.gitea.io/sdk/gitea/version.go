// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

// ServerVersion returns the version of the server
func (c *Client) ServerVersion() (string, *Response, error) {
	var v = struct {
		Version string `json:"version"`
	}{}
	resp, err := c.getParsedResponse("GET", "/version", nil, nil, &v)
	return v.Version, resp, err
}

// CheckServerVersionConstraint validates that the login's server satisfies a
// given version constraint such as ">= 1.11.0+dev"
func (c *Client) CheckServerVersionConstraint(constraint string) error {
	c.versionLock.RLock()
	if c.serverVersion == nil {
		c.versionLock.RUnlock()
		if err := c.loadClientServerVersion(); err != nil {
			return err
		}
	} else {
		c.versionLock.RUnlock()
	}

	check, err := version.NewConstraint(constraint)
	if err != nil {
		return err
	}
	if !check.Check(c.serverVersion) {
		return fmt.Errorf("gitea server at %s does not satisfy version constraint %s", c.url, constraint)
	}
	return nil
}

// loadClientServerVersion init the serverVersion variable
func (c *Client) loadClientServerVersion() error {
	c.versionLock.Lock()
	defer c.versionLock.Unlock()

	raw, _, err := c.ServerVersion()
	if err != nil {
		return err
	}
	if c.serverVersion, err = version.NewVersion(raw); err != nil {
		return err
	}
	return nil
}
