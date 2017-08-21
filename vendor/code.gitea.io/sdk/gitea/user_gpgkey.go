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

// GPGKeyList represents a list of GPGKey
// swagger:response GPGKeyList
type GPGKeyList []*GPGKey

// GPGKey a user GPG key to sign commit and tag in repository
// swagger:response GPGKey
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
// swagger:model GPGKeyEmail
type GPGKeyEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// CreateGPGKeyOption options create user GPG key
// swagger:parameters userCurrentPostGPGKey
type CreateGPGKeyOption struct {
	// An armored GPG key to add
	//
	// in: body
	// required: true
	// unique: true
	ArmoredKey string `json:"armored_public_key" binding:"Required"`
}

// ListGPGKeys list all the GPG keys of the user
func (c *Client) ListGPGKeys(user string) ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 10)
	return keys, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/gpg_keys", user), nil, nil, &keys)
}

// ListMyGPGKeys list all the GPG keys of current user
func (c *Client) ListMyGPGKeys() ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 10)
	return keys, c.getParsedResponse("GET", "/user/gpg_keys", nil, nil, &keys)
}

// GetGPGKey get current user's GPG key by key id
func (c *Client) GetGPGKey(keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	return key, c.getParsedResponse("GET", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil, &key)
}

// CreateGPGKey create GPG key with options
func (c *Client) CreateGPGKey(opt CreateGPGKeyOption) (*GPGKey, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	key := new(GPGKey)
	return key, c.getParsedResponse("POST", "/user/gpg_keys", jsonHeader, bytes.NewReader(body), key)
}

// DeleteGPGKey delete GPG key with key id
func (c *Client) DeleteGPGKey(keyID int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/user/gpg_keys/%d", keyID), nil, nil)
	return err
}
