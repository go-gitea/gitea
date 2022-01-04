// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type DeployKey struct {
	ID       int64     `json:"id"`
	Key      string    `json:"key"`
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Created  time.Time `json:"created_at"`
	ReadOnly bool      `json:"read_only"`
}

func (c *Client) ListDeployKeys(user, repo string) ([]*DeployKey, error) {
	keys := make([]*DeployKey, 0, 10)
	return keys, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/keys", user, repo), nil, nil, &keys)
}

func (c *Client) GetDeployKey(user, repo string, keyID int64) (*DeployKey, error) {
	key := new(DeployKey)
	return key, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/keys/%d", user, repo, keyID), nil, nil, &key)
}

type CreateKeyOption struct {
	Title string `json:"title" binding:"Required"`
	Key   string `json:"key" binding:"Required"`
}

func (c *Client) CreateDeployKey(user, repo string, opt CreateKeyOption) (*DeployKey, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	key := new(DeployKey)
	return key, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/keys", user, repo), jsonHeader, bytes.NewReader(body), key)
}

func (c *Client) DeleteDeployKey(owner, repo string, keyID int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/keys/%d", owner, repo, keyID), nil, nil)
	return err
}
