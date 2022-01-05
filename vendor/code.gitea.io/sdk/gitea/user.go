// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"strconv"
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
	IsAdmin bool `json:"is_admin"`
	// Date and Time of last login
	LastLogin time.Time `json:"last_login"`
	// Date and Time of user creation
	Created time.Time `json:"created"`
	// Is user restricted
	Restricted bool `json:"restricted"`
	// Is user active
	IsActive bool `json:"active"`
	// Is user login prohibited
	ProhibitLogin bool `json:"prohibit_login"`
	// the user's location
	Location string `json:"location"`
	// the user's website
	Website string `json:"website"`
	// the user's description
	Description string `json:"description"`
	// User visibility level option
	Visibility VisibleType `json:"visibility"`

	// user counts
	FollowerCount    int `json:"followers_count"`
	FollowingCount   int `json:"following_count"`
	StarredRepoCount int `json:"starred_repos_count"`
}

// GetUserInfo get user info by user's name
func (c *Client) GetUserInfo(user string) (*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	u := new(User)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s", user), nil, nil, u)
	return u, resp, err
}

// GetMyUserInfo get user info of current user
func (c *Client) GetMyUserInfo() (*User, *Response, error) {
	u := new(User)
	resp, err := c.getParsedResponse("GET", "/user", nil, nil, u)
	return u, resp, err
}

// GetUserByID returns user by a given user ID
func (c *Client) GetUserByID(id int64) (*User, *Response, error) {
	if id < 0 {
		return nil, nil, fmt.Errorf("invalid user id %d", id)
	}

	query := make(url.Values)
	query.Add("uid", strconv.FormatInt(id, 10))
	users, resp, err := c.searchUsers(query.Encode())

	if err != nil {
		return nil, resp, err
	}

	if len(users) == 1 {
		return users[0], resp, err
	}

	return nil, resp, fmt.Errorf("user not found with id %d", id)
}
