// Copyright 2017 Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// GPGKey a user GPG key to sign commit and tag in repository
type GPGKey struct {
	ID                int64          `json:"id"`
	PrimaryKeyID      string         `json:"primary_key_id"`
	KeyID             string         `json:"key_id"`
	PublicKey         string         `json:"public_key"`
	Emails            []*GPGKeyEmail `json:"emails"`
	SubsKey           []*GPGKey      `json:"subkeys"`
	CanSign           bool           `json:"can_sign"`
	CanEncryptComms   bool           `json:"can_encrypt_comms"`
	CanEncryptStorage bool           `json:"can_encrypt_storage"`
	CanCertify        bool           `json:"can_certify"`
	Created           time.Time      `json:"created_at,omitempty"`
	Expires           time.Time      `json:"expires_at,omitempty"`
}

// GPGKeyEmail an email attached to a GPGKey
type GPGKeyEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// ListGPGKeysOptions options for listing a user's GPGKeys
type ListGPGKeysOptions struct {
	ListOptions
}

// ListGPGKeys list all the GPG keys of the user
func (c *Client) ListGPGKeys(user string, opt ListGPGKeysOptions) ([]*GPGKey, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	keys := make([]*GPGKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/gpg_keys?%s", user, opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// ListMyGPGKeys list all the GPG keys of current user
func (c *Client) ListMyGPGKeys(opt *ListGPGKeysOptions) ([]*GPGKey, *Response, error) {
	opt.setDefaults()
	keys := make([]*GPGKey, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/gpg_keys?%s", opt.getURLQuery().Encode()), nil, nil, &keys)
	return keys, resp, err
}

// GetGPGKey get current user's GPG key by key id
func (c *Client) GetGPGKey(keyID int64) (*GPGKey, *Response, error) {
	key := new(GPGKey)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil, &key)
	return key, resp, err
}

// CreateGPGKeyOption options create user GPG key
type CreateGPGKeyOption struct {
	// An armored GPG key to add
	//
	ArmoredKey string `json:"armored_public_key"`
}

// CreateGPGKey create GPG key with options
func (c *Client) CreateGPGKey(opt CreateGPGKeyOption) (*GPGKey, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	key := new(GPGKey)
	resp, err := c.getParsedResponse("POST", "/user/gpg_keys", jsonHeader, bytes.NewReader(body), key)
	return key, resp, err
}

// DeleteGPGKey delete GPG key with key id
func (c *Client) DeleteGPGKey(keyID int64) (*Response, error) {
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil)
	return resp, err
}
