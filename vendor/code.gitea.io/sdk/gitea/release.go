// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Release represents a repository release
type Release struct {
	ID           int64     `json:"id"`
	TagName      string    `json:"tag_name"`
	Target       string    `json:"target_commitish"`
	Title        string    `json:"name"`
	Note         string    `json:"body"`
	URL          string    `json:"url"`
	TarURL       string    `json:"tarball_url"`
	ZipURL       string    `json:"zipball_url"`
	IsDraft      bool      `json:"draft"`
	IsPrerelease bool      `json:"prerelease"`
	// swagger:strfmt date-time
	CreatedAt    time.Time `json:"created_at"`
	// swagger:strfmt date-time
	PublishedAt  time.Time `json:"published_at"`
	Publisher    *User     `json:"author"`
}

// ListReleases list releases of a repository
func (c *Client) ListReleases(user, repo string) ([]*Release, error) {
	releases := make([]*Release, 0, 10)
	err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases", user, repo),
		nil, nil, &releases)
	return releases, err
}

// GetRelease get a release of a repository
func (c *Client) GetRelease(user, repo string, id int64) (*Release, error) {
	r := new(Release)
	err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d", user, repo, id),
		nil, nil, &r)
	return r, err
}

// CreateReleaseOption options when creating a release
type CreateReleaseOption struct {
	// required: true
	TagName      string `json:"tag_name" binding:"Required"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
}

// CreateRelease create a release
func (c *Client) CreateRelease(user, repo string, form CreateReleaseOption) (*Release, error) {
	body, err := json.Marshal(form)
	if err != nil {
		return nil, err
	}
	r := new(Release)
	err = c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/releases", user, repo),
		jsonHeader, bytes.NewReader(body), r)
	return r, err
}

// EditReleaseOption options when editing a release
type EditReleaseOption struct {
	TagName      string `json:"tag_name"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      *bool  `json:"draft"`
	IsPrerelease *bool  `json:"prerelease"`
}

// EditRelease edit a release
func (c *Client) EditRelease(user, repo string, id int64, form EditReleaseOption) (*Release, error) {
	body, err := json.Marshal(form)
	if err != nil {
		return nil, err
	}
	r := new(Release)
	err = c.getParsedResponse("PATCH",
		fmt.Sprintf("/repos/%s/%s/releases/%d", user, repo, id),
		jsonHeader, bytes.NewReader(body), r)
	return r, err
}

// DeleteRelease delete a release from a repository
func (c *Client) DeleteRelease(user, repo string, id int64) error {
	_, err := c.getResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/releases/%d", user, repo, id),
		nil, nil)
	return err
}
