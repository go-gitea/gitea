// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import "fmt"

// ListFollowersOptions options for listing followers
type ListFollowersOptions struct {
	ListOptions
}

// ListMyFollowers list all the followers of current user
func (c *Client) ListMyFollowers(opt ListFollowersOptions) ([]*User, *Response, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/followers?%s", opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowers list all the followers of one user
func (c *Client) ListFollowers(user string, opt ListFollowersOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/followers?%s", user, opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowingOptions options for listing a user's users being followed
type ListFollowingOptions struct {
	ListOptions
}

// ListMyFollowing list all the users current user followed
func (c *Client) ListMyFollowing(opt ListFollowingOptions) ([]*User, *Response, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/following?%s", opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowing list all the users the user followed
func (c *Client) ListFollowing(user string, opt ListFollowingOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/following?%s", user, opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// IsFollowing if current user followed the target
func (c *Client) IsFollowing(target string) (bool, *Response) {
	if err := escapeValidatePathSegments(&target); err != nil {
		// ToDo return err
		return false, nil
	}
	_, resp, err := c.getResponse("GET", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return err == nil, resp
}

// IsUserFollowing if the user followed the target
func (c *Client) IsUserFollowing(user, target string) (bool, *Response) {
	if err := escapeValidatePathSegments(&user, &target); err != nil {
		// ToDo return err
		return false, nil
	}
	_, resp, err := c.getResponse("GET", fmt.Sprintf("/users/%s/following/%s", user, target), nil, nil)
	return err == nil, resp
}

// Follow set current user follow the target
func (c *Client) Follow(target string) (*Response, error) {
	if err := escapeValidatePathSegments(&target); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return resp, err
}

// Unfollow set current user unfollow the target
func (c *Client) Unfollow(target string) (*Response, error) {
	if err := escapeValidatePathSegments(&target); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return resp, err
}
