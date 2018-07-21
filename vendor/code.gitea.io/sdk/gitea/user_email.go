// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
)

// Email an email address belonging to a user
type Email struct {
	// swagger:strfmt email
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Primary  bool   `json:"primary"`
}

// ListEmails all the email addresses of user
func (c *Client) ListEmails() ([]*Email, error) {
	emails := make([]*Email, 0, 3)
	return emails, c.getParsedResponse("GET", "/user/emails", nil, nil, &emails)
}

// CreateEmailOption options when creating email addresses
type CreateEmailOption struct {
	// email addresses to add
	Emails []string `json:"emails"`
}

// AddEmail add one email to current user with options
func (c *Client) AddEmail(opt CreateEmailOption) ([]*Email, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	emails := make([]*Email, 0, 3)
	return emails, c.getParsedResponse("POST", "/user/emails", jsonHeader, bytes.NewReader(body), emails)
}

// DeleteEmailOption options when deleting email addresses
type DeleteEmailOption struct {
	// email addresses to delete
	Emails []string `json:"emails"`
}

// DeleteEmail delete one email of current users'
func (c *Client) DeleteEmail(opt DeleteEmailOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("DELETE", "/user/emails", jsonHeader, bytes.NewReader(body))
	return err
}
