// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// TrackedTime worked time for an issue / pr
type TrackedTime struct {
	ID      int64     `json:"id"`
	Created time.Time `json:"created"`
	// Time in seconds
	Time int64 `json:"time"`
	// deprecated (only for backwards compatibility)
	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
	// deprecated (only for backwards compatibility)
	IssueID int64  `json:"issue_id"`
	Issue   *Issue `json:"issue"`
}

// ListTrackedTimesOptions options for listing repository's tracked times
type ListTrackedTimesOptions struct {
	ListOptions
	Since  time.Time
	Before time.Time
	// User filter is only used by ListRepoTrackedTimes !!!
	User string
}

// QueryEncode turns options into querystring argument
func (opt *ListTrackedTimesOptions) QueryEncode() string {
	query := opt.getURLQuery()

	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}

	if len(opt.User) != 0 {
		query.Add("user", opt.User)
	}

	return query.Encode()
}

// ListRepoTrackedTimes list tracked times of a repository
func (c *Client) ListRepoTrackedTimes(owner, repo string, opt ListTrackedTimesOptions) ([]*TrackedTime, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/times", owner, repo))
	opt.setDefaults()
	link.RawQuery = opt.QueryEncode()
	times := make([]*TrackedTime, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &times)
	return times, resp, err
}

// GetMyTrackedTimes list tracked times of the current user
func (c *Client) GetMyTrackedTimes() ([]*TrackedTime, *Response, error) {
	times := make([]*TrackedTime, 0, 10)
	resp, err := c.getParsedResponse("GET", "/user/times", jsonHeader, nil, &times)
	return times, resp, err
}

// AddTimeOption options for adding time to an issue
type AddTimeOption struct {
	// time in seconds
	Time int64 `json:"time"`
	// optional
	Created time.Time `json:"created"`
	// optional
	User string `json:"user_name"`
}

// Validate the AddTimeOption struct
func (opt AddTimeOption) Validate() error {
	if opt.Time == 0 {
		return fmt.Errorf("no time to add")
	}
	return nil
}

// AddTime adds time to issue with the given index
func (c *Client) AddTime(owner, repo string, index int64, opt AddTimeOption) (*TrackedTime, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	t := new(TrackedTime)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/times", owner, repo, index),
		jsonHeader, bytes.NewReader(body), t)
	return t, resp, err
}

// ListIssueTrackedTimes list tracked times of a single issue for a given repository
func (c *Client) ListIssueTrackedTimes(owner, repo string, index int64, opt ListTrackedTimesOptions) ([]*TrackedTime, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/times", owner, repo, index))
	opt.setDefaults()
	link.RawQuery = opt.QueryEncode()
	times := make([]*TrackedTime, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &times)
	return times, resp, err
}

// ResetIssueTime reset tracked time of a single issue for a given repository
func (c *Client) ResetIssueTime(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/times", owner, repo, index), jsonHeader, nil)
	return resp, err
}

// DeleteTime delete a specific tracked time by id of a single issue for a given repository
func (c *Client) DeleteTime(owner, repo string, index, timeID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/times/%d", owner, repo, index, timeID), jsonHeader, nil)
	return resp, err
}
