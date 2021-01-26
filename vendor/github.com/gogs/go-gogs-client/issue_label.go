// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	URL   string `json:"url"`
}

func (c *Client) ListRepoLabels(owner, repo string) ([]*Label, error) {
	labels := make([]*Label, 0, 10)
	return labels, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels", owner, repo), nil, nil, &labels)
}

func (c *Client) GetRepoLabel(owner, repo string, id int64) (*Label, error) {
	label := new(Label)
	return label, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil, label)
}

type CreateLabelOption struct {
	Name  string `json:"name" binding:"Required"`
	Color string `json:"color" binding:"Required;Size(7)"`
}

func (c *Client) CreateLabel(owner, repo string, opt CreateLabelOption) (*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	label := new(Label)
	return label, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/labels", owner, repo),
		jsonHeader, bytes.NewReader(body), label)
}

type EditLabelOption struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

func (c *Client) EditLabel(owner, repo string, id int64, opt EditLabelOption) (*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	label := new(Label)
	return label, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), label)
}

func (c *Client) DeleteLabel(owner, repo string, id int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil)
	return err
}

type IssueLabelsOption struct {
	Labels []int64 `json:"labels"`
}

func (c *Client) GetIssueLabels(owner, repo string, index int64) ([]*Label, error) {
	labels := make([]*Label, 0, 5)
	return labels, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), nil, nil, &labels)
}

func (c *Client) AddIssueLabels(owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	labels := make([]*Label, 0)
	return labels, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
}

func (c *Client) ReplaceIssueLabels(owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	labels := make([]*Label, 0)
	return labels, c.getParsedResponse("PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
}

func (c *Client) DeleteIssueLabel(owner, repo string, index, label int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%d", owner, repo, index, label), nil, nil)
	return err
}

func (c *Client) ClearIssueLabels(owner, repo string, index int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), nil, nil)
	return err
}
