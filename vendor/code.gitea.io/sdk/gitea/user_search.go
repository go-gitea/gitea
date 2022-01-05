// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
)

type searchUsersResponse struct {
	Users []*User `json:"data"`
}

// SearchUsersOption options for SearchUsers
type SearchUsersOption struct {
	ListOptions
	KeyWord string
}

// QueryEncode turns options into querystring argument
func (opt *SearchUsersOption) QueryEncode() string {
	query := make(url.Values)
	if opt.Page > 0 {
		query.Add("page", fmt.Sprintf("%d", opt.Page))
	}
	if opt.PageSize > 0 {
		query.Add("limit", fmt.Sprintf("%d", opt.PageSize))
	}
	if len(opt.KeyWord) > 0 {
		query.Add("q", opt.KeyWord)
	}
	return query.Encode()
}

func (c *Client) searchUsers(rawQuery string) ([]*User, *Response, error) {
	link, _ := url.Parse("/users/search")
	link.RawQuery = rawQuery
	userResp := new(searchUsersResponse)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &userResp)
	return userResp.Users, resp, err
}

// SearchUsers finds users by query
func (c *Client) SearchUsers(opt SearchUsersOption) ([]*User, *Response, error) {
	return c.searchUsers(opt.QueryEncode())
}
