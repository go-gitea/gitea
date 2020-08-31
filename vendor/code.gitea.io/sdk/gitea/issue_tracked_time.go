// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// GetUserTrackedTimes list tracked times of a user
func (c *Client) GetUserTrackedTimes(owner, repo, user string) ([]*TrackedTime, error) {
	times := make([]*TrackedTime, 0, 10)
	return times, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/times/%s", owner, repo, user), nil, nil, &times)
}

// GetRepoTrackedTimes list tracked times of a repository
func (c *Client) GetRepoTrackedTimes(owner, repo string) ([]*TrackedTime, error) {
	times := make([]*TrackedTime, 0, 10)
	return times, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/times", owner, repo), nil, nil, &times)
}

// GetMyTrackedTimes list tracked times of the current user
func (c *Client) GetMyTrackedTimes() ([]*TrackedTime, error) {
	times := make([]*TrackedTime, 0, 10)
	return times, c.getParsedResponse("GET", "/user/times", nil, nil, &times)
}

// AddTimeOption options for adding time to an issue
type AddTimeOption struct {
	// time in seconds
	Time int64 `json:"time" binding:"Required"`
	// optional
	Created time.Time `json:"created"`
	// optional
	User string `json:"user_name"`
}

// AddTime adds time to issue with the given index
func (c *Client) AddTime(owner, repo string, index int64, opt AddTimeOption) (*TrackedTime, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	t := new(TrackedTime)
	return t, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/times", owner, repo, index),
		jsonHeader, bytes.NewReader(body), t)
}

// ListTrackedTimesOptions options for listing repository's tracked times
type ListTrackedTimesOptions struct {
	ListOptions
}

// ListTrackedTimes list tracked times of a single issue for a given repository
func (c *Client) ListTrackedTimes(owner, repo string, index int64, opt ListTrackedTimesOptions) ([]*TrackedTime, error) {
	opt.setDefaults()
	times := make([]*TrackedTime, 0, opt.PageSize)
	return times, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/times?%s", owner, repo, index, opt.getURLQuery().Encode()), nil, nil, &times)
}

// ResetIssueTime reset tracked time of a single issue for a given repository
func (c *Client) ResetIssueTime(owner, repo string, index int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/times", owner, repo, index), nil, nil)
	return err
}

// DeleteTime delete a specific tracked time by id of a single issue for a given repository
func (c *Client) DeleteTime(owner, repo string, index, timeID int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/times/%d", owner, repo, index, timeID), nil, nil)
	return err
}
