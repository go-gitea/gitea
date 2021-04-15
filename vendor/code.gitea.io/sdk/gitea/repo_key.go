// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// DeployKey a deploy key
type DeployKey struct {
	ID          int64       `json:"id"`
	KeyID       int64       `json:"key_id"`
	Key         string      `json:"key"`
	URL         string      `json:"url"`
	Title       string      `json:"title"`
	Fingerprint string      `json:"fingerprint"`
	Created     time.Time   `json:"created_at"`
	ReadOnly    bool        `json:"read_only"`
	Repository  *Repository `json:"repository,omitempty"`
}

// ListDeployKeysOptions options for listing a repository's deploy keys
type ListDeployKeysOptions struct {
	ListOptions
	KeyID       int64
	Fingerprint string
}

// QueryEncode turns options into querystring argument
func (opt *ListDeployKeysOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.KeyID > 0 {
		query.Add("key_id", fmt.Sprintf("%d", opt.KeyID))
	}
	if len(opt.Fingerprint) > 0 {
		query.Add("fingerprint", opt.Fingerprint)
	}
	return query.Encode()
}

// ListDeployKeys list all the deploy keys of one repository
func (c *Client) ListDeployKeys(user, repo string, opt ListDeployKeysOptions) ([]*DeployKey, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/keys", user, repo))
	opt.setDefaults()
	link.RawQuery = opt.QueryEncode()
	keys := make([]*DeployKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &keys)
	return keys, resp, err
}

// GetDeployKey get one deploy key with key id
func (c *Client) GetDeployKey(user, repo string, keyID int64) (*DeployKey, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	key := new(DeployKey)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/keys/%d", user, repo, keyID), nil, nil, &key)
	return key, resp, err
}

// CreateDeployKey options when create one deploy key
func (c *Client) CreateDeployKey(user, repo string, opt CreateKeyOption) (*DeployKey, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(DeployKey)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/keys", user, repo), jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}

// DeleteDeployKey delete deploy key with key id
func (c *Client) DeleteDeployKey(owner, repo string, keyID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/keys/%d", owner, repo, keyID), nil, nil)
	return resp, err
}
