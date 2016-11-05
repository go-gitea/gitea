// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import "fmt"

func (c *Client) ListMyFollowers(page int) ([]*User, error) {
	users := make([]*User, 0, 10)
	return users, c.getParsedResponse("GET", fmt.Sprintf("/user/followers?page=%d", page), nil, nil, &users)
}

func (c *Client) ListFollowers(user string, page int) ([]*User, error) {
	users := make([]*User, 0, 10)
	return users, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/followers?page=%d", user, page), nil, nil, &users)
}

func (c *Client) ListMyFollowing(page int) ([]*User, error) {
	users := make([]*User, 0, 10)
	return users, c.getParsedResponse("GET", fmt.Sprintf("/user/following?page=%d", page), nil, nil, &users)
}

func (c *Client) ListFollowing(user string, page int) ([]*User, error) {
	users := make([]*User, 0, 10)
	return users, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/following?page=%d", user, page), nil, nil, &users)
}

func (c *Client) IsFollowing(target string) bool {
	_, err := c.getResponse("GET", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return err == nil
}

func (c *Client) IsUserFollowing(user, target string) bool {
	_, err := c.getResponse("GET", fmt.Sprintf("/users/%s/following/%s", user, target), nil, nil)
	return err == nil
}

func (c *Client) Follow(target string) error {
	_, err := c.getResponse("PUT", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return err
}

func (c *Client) Unfollow(target string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return err
}
