// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Milestone milestone is a collection of issues on one repository
type Milestone struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	State        StateType  `json:"state"`
	OpenIssues   int        `json:"open_issues"`
	ClosedIssues int        `json:"closed_issues"`
	Created      time.Time  `json:"created_at"`
	Updated      *time.Time `json:"updated_at"`
	Closed       *time.Time `json:"closed_at"`
	Deadline     *time.Time `json:"due_on"`
}

// ListMilestoneOption list milestone options
type ListMilestoneOption struct {
	ListOptions
	// open, closed, all
	State StateType
	Name  string
}

// QueryEncode turns options into querystring argument
func (opt *ListMilestoneOption) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.State != "" {
		query.Add("state", string(opt.State))
	}
	if len(opt.Name) != 0 {
		query.Add("name", opt.Name)
	}
	return query.Encode()
}

// ListRepoMilestones list all the milestones of one repository
func (c *Client) ListRepoMilestones(owner, repo string, opt ListMilestoneOption) ([]*Milestone, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	milestones := make([]*Milestone, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/milestones", owner, repo))
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &milestones)
	return milestones, resp, err
}

// GetMilestone get one milestone by repo name and milestone id
func (c *Client) GetMilestone(owner, repo string, id int64) (*Milestone, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	milestone := new(Milestone)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), nil, nil, milestone)
	return milestone, resp, err
}

// GetMilestoneByName get one milestone by repo and milestone name
func (c *Client) GetMilestoneByName(owner, repo string, name string) (*Milestone, *Response, error) {
	if c.CheckServerVersionConstraint(">=1.13") != nil {
		// backwards compatibility mode
		m, resp, err := c.resolveMilestoneByName(owner, repo, name)
		return m, resp, err
	}
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	milestone := new(Milestone)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/milestones/%s", owner, repo, name), nil, nil, milestone)
	return milestone, resp, err
}

// CreateMilestoneOption options for creating a milestone
type CreateMilestoneOption struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	State       StateType  `json:"state"`
	Deadline    *time.Time `json:"due_on"`
}

// Validate the CreateMilestoneOption struct
func (opt CreateMilestoneOption) Validate() error {
	if len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	return nil
}

// CreateMilestone create one milestone with options
func (c *Client) CreateMilestone(owner, repo string, opt CreateMilestoneOption) (*Milestone, *Response, error) {
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
	milestone := new(Milestone)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/milestones", owner, repo), jsonHeader, bytes.NewReader(body), milestone)

	// make creating closed milestones need gitea >= v1.13.0
	// this make it backwards compatible
	if err == nil && opt.State == StateClosed && milestone.State != StateClosed {
		closed := StateClosed
		return c.EditMilestone(owner, repo, milestone.ID, EditMilestoneOption{
			State: &closed,
		})
	}

	return milestone, resp, err
}

// EditMilestoneOption options for editing a milestone
type EditMilestoneOption struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	State       *StateType `json:"state"`
	Deadline    *time.Time `json:"due_on"`
}

// Validate the EditMilestoneOption struct
func (opt EditMilestoneOption) Validate() error {
	if len(opt.Title) != 0 && len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	return nil
}

// EditMilestone modify milestone with options
func (c *Client) EditMilestone(owner, repo string, id int64, opt EditMilestoneOption) (*Milestone, *Response, error) {
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
	milestone := new(Milestone)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), milestone)
	return milestone, resp, err
}

// EditMilestoneByName modify milestone with options
func (c *Client) EditMilestoneByName(owner, repo string, name string, opt EditMilestoneOption) (*Milestone, *Response, error) {
	if c.CheckServerVersionConstraint(">=1.13") != nil {
		// backwards compatibility mode
		m, _, err := c.resolveMilestoneByName(owner, repo, name)
		if err != nil {
			return nil, nil, err
		}
		return c.EditMilestone(owner, repo, m.ID, opt)
	}
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	milestone := new(Milestone)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/milestones/%s", owner, repo, name), jsonHeader, bytes.NewReader(body), milestone)
	return milestone, resp, err
}

// DeleteMilestone delete one milestone by id
func (c *Client) DeleteMilestone(owner, repo string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/milestones/%d", owner, repo, id), nil, nil)
	return resp, err
}

// DeleteMilestoneByName delete one milestone by name
func (c *Client) DeleteMilestoneByName(owner, repo string, name string) (*Response, error) {
	if c.CheckServerVersionConstraint(">=1.13") != nil {
		// backwards compatibility mode
		m, _, err := c.resolveMilestoneByName(owner, repo, name)
		if err != nil {
			return nil, err
		}
		return c.DeleteMilestone(owner, repo, m.ID)
	}
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/milestones/%s", owner, repo, name), nil, nil)
	return resp, err
}

// resolveMilestoneByName is a fallback method to find milestone id by name
func (c *Client) resolveMilestoneByName(owner, repo, name string) (*Milestone, *Response, error) {
	for i := 1; ; i++ {
		miles, resp, err := c.ListRepoMilestones(owner, repo, ListMilestoneOption{
			ListOptions: ListOptions{
				Page: i,
			},
			State: "all",
		})
		if err != nil {
			return nil, nil, err
		}
		if len(miles) == 0 {
			return nil, nil, fmt.Errorf("milestone '%s' do not exist", name)
		}
		for _, m := range miles {
			if strings.ToLower(strings.TrimSpace(m.Title)) == strings.ToLower(strings.TrimSpace(name)) {
				return m, resp, nil
			}
		}
	}
}
