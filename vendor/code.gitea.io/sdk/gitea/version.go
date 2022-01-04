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
	if err := c.loadServerVersion(); err != nil {
		return err
	}

	check, err := version.NewConstraint(constraint)
	if err != nil {
		return err
	}
	if !check.Check(c.serverVersion) {
		c.mutex.RLock()
		url := c.url
		c.mutex.RUnlock()
		return fmt.Errorf("gitea server at %s does not satisfy version constraint %s", url, constraint)
	}
	return nil
}

// predefined versions only have to be parsed by library once
var (
	version1_11_0, _ = version.NewVersion("1.11.0")
	version1_12_0, _ = version.NewVersion("1.12.0")
	version1_13_0, _ = version.NewVersion("1.13.0")
	version1_14_0, _ = version.NewVersion("1.14.0")
)

// checkServerVersionGreaterThanOrEqual is internally used to speed up things and ignore issues with prerelease
func (c *Client) checkServerVersionGreaterThanOrEqual(v *version.Version) error {
	if err := c.loadServerVersion(); err != nil {
		return err
	}

	if !c.serverVersion.GreaterThanOrEqual(v) {
		c.mutex.RLock()
		url := c.url
		c.mutex.RUnlock()
		return fmt.Errorf("gitea server at %s is older than %s", url, v.Original())
	}
	return nil
}

// loadServerVersion init the serverVersion variable
func (c *Client) loadServerVersion() (err error) {
	c.getVersionOnce.Do(func() {
		raw, _, err2 := c.ServerVersion()
		if err2 != nil {
			err = err2
			return
		}
		if c.serverVersion, err = version.NewVersion(raw); err != nil {
			return
		}
	})
	return
}
