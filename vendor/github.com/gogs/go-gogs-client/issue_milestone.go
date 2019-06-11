// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type Milestone struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	State        StateType  `json:"state"`
	OpenIssues   int        `json:"open_issues"`
	ClosedIssues int        `json:"closed_issues"`
	Closed       *time.Time `json:"closed_at"`
	Deadline     *time.Time `json:"due_on"`
}

func (c *Client) ListRepoMilestones(owner, repo string) ([]*Milestone, error) {
	milestones := make([]*Milestone, 0, 10)
	return milestones, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), nil, nil, &milestones)
}

func (c *Client) GetMilestone(owner, repo string, id int64) (*Milestone, error) {
	milestone := new(Milestone)
	return milestone, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), nil, nil, milestone)
}

type CreateMilestoneOption struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Deadline    *time.Time `json:"due_on"`
}

func (c *Client) CreateMilestone(owner, repo string, opt CreateMilestoneOption) (*Milestone, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	milestone := new(Milestone)
	return milestone, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), jsonHeader, bytes.NewReader(body), milestone)
}

type EditMilestoneOption struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	State       *string    `json:"state"`
	Deadline    *time.Time `json:"due_on"`
}

func (c *Client) EditMilestone(owner, repo string, id int64, opt EditMilestoneOption) (*Milestone, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	milestone := new(Milestone)
	return milestone, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), milestone)
}

func (c *Client) DeleteMilestone(owner, repo string, id int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), nil, nil)
	return err
}
