// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// PublicKey publickey is a user key to push code to repository
type PublicKey struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	URL         string    `json:"url,omitempty"`
	Title       string    `json:"title,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Created     time.Time `json:"created_at,omitempty"`
	Owner       *User     `json:"user,omitempty"`
	ReadOnly    bool      `json:"read_only,omitempty"`
	KeyType     string    `json:"key_type,omitempty"`
}

// ListPublicKeysOptions options for listing a user's PublicKeys
type ListPublicKeysOptions struct {
	ListOptions
}

// ListPublicKeys list all the public keys of the user
func (c *Client) ListPublicKeys(user string, opt ListPublicKeysOptions) ([]*PublicKey, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	keys := make([]*PublicKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/keys?%s", user, opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// ListMyPublicKeys list all the public keys of current user
func (c *Client) ListMyPublicKeys(opt ListPublicKeysOptions) ([]*PublicKey, *Response, error) {
	opt.setDefaults()
	keys := make([]*PublicKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/keys?%s", opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// GetPublicKey get current user's public key by key id
func (c *Client) GetPublicKey(keyID int64) (*PublicKey, *Response, error) {
	key := new(PublicKey)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/keys/%d", keyID), nil, nil, &key)
	return key, resp, err
}

// CreateKeyOption options when creating a key
type CreateKeyOption struct {
	// Title of the key to add
	Title string `json:"title"`
	// An armored SSH key to add
	Key string `json:"key"`
	// Describe if the key has only read access or read/write
	ReadOnly bool `json:"read_only"`
}

// CreatePublicKey create public key with options
func (c *Client) CreatePublicKey(opt CreateKeyOption) (*PublicKey, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(PublicKey)
	resp, err := c.getParsedResponse("POST", "/user/keys", jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}

// DeletePublicKey delete public key with key id
func (c *Client) DeletePublicKey(keyID int64) (*Response, error) {
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/user/keys/%d", keyID), nil, nil)
	return resp, err
}
