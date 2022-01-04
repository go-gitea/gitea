// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Oauth2 represents an Oauth2 Application
type Oauth2 struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RedirectURIs []string  `json:"redirect_uris"`
	Created      time.Time `json:"created"`
}

// ListOauth2Option for listing Oauth2 Applications
type ListOauth2Option struct {
	ListOptions
}

// CreateOauth2Option required options for creating an Application
type CreateOauth2Option struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
}

// CreateOauth2 create an Oauth2 Application and returns a completed Oauth2 object.
func (c *Client) CreateOauth2(opt CreateOauth2Option) (*Oauth2, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	oauth := new(Oauth2)
	resp, err := c.getParsedResponse("POST", "/user/applications/oauth2", jsonHeader, bytes.NewReader(body), oauth)
	return oauth, resp, err
}

// UpdateOauth2 a specific Oauth2 Application by ID and return a completed Oauth2 object.
func (c *Client) UpdateOauth2(oauth2id int64, opt CreateOauth2Option) (*Oauth2, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	oauth := new(Oauth2)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), jsonHeader, bytes.NewReader(body), oauth)
	return oauth, resp, err
}

// GetOauth2 a specific Oauth2 Application by ID.
func (c *Client) GetOauth2(oauth2id int64) (*Oauth2, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	oauth2s := &Oauth2{}
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), nil, nil, &oauth2s)
	return oauth2s, resp, err
}

// ListOauth2 all of your Oauth2 Applications.
func (c *Client) ListOauth2(opt ListOauth2Option) ([]*Oauth2, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	oauth2s := make([]*Oauth2, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/applications/oauth2?%s", opt.getURLQuery().Encode()), nil, nil, &oauth2s)
	return oauth2s, resp, err
}

// DeleteOauth2 delete an Oauth2 application by ID
func (c *Client) DeleteOauth2(oauth2id int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), nil, nil)
	return resp, err
}
