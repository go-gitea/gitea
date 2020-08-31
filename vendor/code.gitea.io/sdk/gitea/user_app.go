// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// AccessToken represents an API access token.
type AccessToken struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Token          string `json:"sha1"`
	TokenLastEight string `json:"token_last_eight"`
}

// ListAccessTokensOptions options for listing a users's access tokens
type ListAccessTokensOptions struct {
	ListOptions
}

// ListAccessTokens lists all the access tokens of user
func (c *Client) ListAccessTokens(opts ListAccessTokensOptions) ([]*AccessToken, error) {
	if len(c.username) == 0 {
		return nil, fmt.Errorf("\"username\" not set: only BasicAuth allowed")
	}
	opts.setDefaults()
	tokens := make([]*AccessToken, 0, opts.PageSize)
	return tokens, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/tokens?%s", c.username, opts.getURLQuery().Encode()), jsonHeader, nil, &tokens)
}

// CreateAccessTokenOption options when create access token
type CreateAccessTokenOption struct {
	Name string `json:"name"`
}

// CreateAccessToken create one access token with options
func (c *Client) CreateAccessToken(opt CreateAccessTokenOption) (*AccessToken, error) {
	if len(c.username) == 0 {
		return nil, fmt.Errorf("\"username\" not set: only BasicAuth allowed")
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	t := new(AccessToken)
	return t, c.getParsedResponse("POST", fmt.Sprintf("/users/%s/tokens", c.username), jsonHeader, bytes.NewReader(body), t)
}

// DeleteAccessToken delete token with key id
func (c *Client) DeleteAccessToken(keyID int64) error {
	if len(c.username) == 0 {
		return fmt.Errorf("\"username\" not set: only BasicAuth allowed")
	}
	_, err := c.getResponse("DELETE", fmt.Sprintf("/users/%s/tokens/%d", c.username, keyID), jsonHeader, nil)
	return err
}
