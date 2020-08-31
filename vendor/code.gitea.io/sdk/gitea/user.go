// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"time"
)

// User represents a user
type User struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's username
	UserName string `json:"login"`
	// the user's full name
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// User locale
	Language string `json:"language"`
	// Is the user an administrator
	IsAdmin   bool      `json:"is_admin"`
	LastLogin time.Time `json:"last_login,omitempty"`
	Created   time.Time `json:"created,omitempty"`
}

// GetUserInfo get user info by user's name
func (c *Client) GetUserInfo(user string) (*User, error) {
	u := new(User)
	err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s", user), nil, nil, u)
	return u, err
}

// GetMyUserInfo get user info of current user
func (c *Client) GetMyUserInfo() (*User, error) {
	u := new(User)
	err := c.getParsedResponse("GET", "/user", nil, nil, u)
	return u, err
}
