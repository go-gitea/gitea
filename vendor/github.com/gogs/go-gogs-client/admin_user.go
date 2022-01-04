// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type CreateUserOption struct {
	SourceID   int64  `json:"source_id"`
	LoginName  string `json:"login_name"`
	Username   string `json:"username" binding:"Required;AlphaDashDot;MaxSize(35)"`
	FullName   string `json:"full_name" binding:"MaxSize(100)"`
	Email      string `json:"email" binding:"Required;Email;MaxSize(254)"`
	Password   string `json:"password" binding:"MaxSize(255)"`
	SendNotify bool   `json:"send_notify"`
}

func (c *Client) AdminCreateUser(opt CreateUserOption) (*User, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	user := new(User)
	return user, c.getParsedResponse("POST", "/admin/users", jsonHeader, bytes.NewReader(body), user)
}

type EditUserOption struct {
	SourceID         int64  `json:"source_id"`
	LoginName        string `json:"login_name"`
	FullName         string `json:"full_name" binding:"MaxSize(100)"`
	Email            string `json:"email" binding:"Required;Email;MaxSize(254)"`
	Password         string `json:"password" binding:"MaxSize(255)"`
	Website          string `json:"website" binding:"MaxSize(50)"`
	Location         string `json:"location" binding:"MaxSize(50)"`
	Active           *bool  `json:"active"`
	Admin            *bool  `json:"admin"`
	AllowGitHook     *bool  `json:"allow_git_hook"`
	AllowImportLocal *bool  `json:"allow_import_local"`
	MaxRepoCreation  *int   `json:"max_repo_creation"`
}

func (c *Client) AdminEditUser(user string, opt EditUserOption) error {
	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}
	_, err = c.getResponse("PATCH", fmt.Sprintf("/admin/users/%s", user), jsonHeader, bytes.NewReader(body))
	return err
}

func (c *Client) AdminDeleteUser(user string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/admin/users/%s", user), nil, nil)
	return err
}

func (c *Client) AdminCreateUserPublicKey(user string, opt CreateKeyOption) (*PublicKey, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	key := new(PublicKey)
	return key, c.getParsedResponse("POST", fmt.Sprintf("/admin/users/%s/keys", user), jsonHeader, bytes.NewReader(body), key)
}
