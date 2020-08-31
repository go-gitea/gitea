// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// AdminListUsersOptions options for listing admin users
type AdminListUsersOptions struct {
	ListOptions
}

// AdminListUsers lists all users
func (c *Client) AdminListUsers(opt AdminListUsersOptions) ([]*User, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	return users, c.getParsedResponse("GET", fmt.Sprintf("/admin/users?%s", opt.getURLQuery().Encode()), nil, nil, &users)
}

// CreateUserOption create user options
type CreateUserOption struct {
	SourceID           int64  `json:"source_id"`
	LoginName          string `json:"login_name"`
	Username           string `json:"username"`
	FullName           string `json:"full_name"`
	Email              string `json:"email"`
	Password           string `json:"password"`
	MustChangePassword *bool  `json:"must_change_password"`
	SendNotify         bool   `json:"send_notify"`
}

// AdminCreateUser create a user
func (c *Client) AdminCreateUser(opt CreateUserOption) (*User, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	user := new(User)
	return user, c.getParsedResponse("POST", "/admin/users", jsonHeader, bytes.NewReader(body), user)
}

// EditUserOption edit user options
type EditUserOption struct {
	SourceID                int64  `json:"source_id"`
	LoginName               string `json:"login_name"`
	FullName                string `json:"full_name"`
	Email                   string `json:"email"`
	Password                string `json:"password"`
	MustChangePassword      *bool  `json:"must_change_password"`
	Website                 string `json:"website"`
	Location                string `json:"location"`
	Active                  *bool  `json:"active"`
	Admin                   *bool  `json:"admin"`
	AllowGitHook            *bool  `json:"allow_git_hook"`
	AllowImportLocal        *bool  `json:"allow_import_local"`
	MaxRepoCreation         *int   `json:"max_repo_creation"`
	ProhibitLogin           *bool  `json:"prohibit_login"`
	AllowCreateOrganization *bool  `json:"allow_create_organization"`
}

// AdminEditUser modify user informations
func (c *Client) AdminEditUser(user string, opt EditUserOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/admin/users/%s", user), jsonHeader, bytes.NewReader(body))
	return err
}

// AdminDeleteUser delete one user according name
func (c *Client) AdminDeleteUser(user string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/admin/users/%s", user), nil, nil)
	return err
}

// AdminCreateUserPublicKey adds a public key for the user
func (c *Client) AdminCreateUserPublicKey(user string, opt CreateKeyOption) (*PublicKey, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	key := new(PublicKey)
	return key, c.getParsedResponse("POST", fmt.Sprintf("/admin/users/%s/keys", user), jsonHeader, bytes.NewReader(body), key)
}

// AdminDeleteUserPublicKey deletes a user's public key
func (c *Client) AdminDeleteUserPublicKey(user string, keyID int) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/admin/users/%s/keys/%d", user, keyID), nil, nil)
	return err
}
