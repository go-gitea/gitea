// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"time"
)

// StopWatch represents a running stopwatch of an issue / pr
type StopWatch struct {
	Created    time.Time `json:"created"`
	IssueIndex int64     `json:"issue_index"`
}

// GetMyStopwatches list all stopwatches
func (c *Client) GetMyStopwatches() ([]*StopWatch, *Response, error) {
	stopwatches := make([]*StopWatch, 0, 1)
	resp, err := c.getParsedResponse("GET", "/user/stopwatches", nil, nil, &stopwatches)
	return stopwatches, resp, err
}

// DeleteIssueStopwatch delete / cancel a specific stopwatch
func (c *Client) DeleteIssueStopwatch(owner, repo string, index int64) (*Response, error) {
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/delete", owner, repo, index), nil, nil)
	return resp, err
}

// StartIssueStopWatch starts a stopwatch for an existing issue for a given
// repository
func (c *Client) StartIssueStopWatch(owner, repo string, index int64) (*Response, error) {
	_, resp, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/start", owner, repo, index), nil, nil)
	return resp, err
}

// StopIssueStopWatch stops an existing stopwatch for an issue in a given
// repository
func (c *Client) StopIssueStopWatch(owner, repo string, index int64) (*Response, error) {
	_, resp, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/stop", owner, repo, index), nil, nil)
	return resp, err
}
