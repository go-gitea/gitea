// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
)

type Email struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Primary  bool   `json:"primary"`
}

func (c *Client) ListEmails() ([]*Email, error) {
	emails := make([]*Email, 0, 3)
	return emails, c.getParsedResponse("GET", "/user/emails", nil, nil, &emails)
}

type CreateEmailOption struct {
	Emails []string `json:"emails"`
}

func (c *Client) AddEmail(opt CreateEmailOption) ([]*Email, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	emails := make([]*Email, 0, 3)
	return emails, c.getParsedResponse("POST", "/user/emails", jsonHeader, bytes.NewReader(body), emails)
}

func (c *Client) DeleteEmail(opt CreateEmailOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("DELETE", "/user/emails", jsonHeader, bytes.NewReader(body))
	return err
}
