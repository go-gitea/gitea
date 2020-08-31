// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Label a label to an issue or a pr
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// example: 00aabb
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// ListLabelsOptions options for listing repository's labels
type ListLabelsOptions struct {
	ListOptions
}

// ListRepoLabels list labels of one repository
func (c *Client) ListRepoLabels(owner, repo string, opt ListLabelsOptions) ([]*Label, error) {
	opt.setDefaults()
	labels := make([]*Label, 0, opt.PageSize)
	return labels, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels?%s", owner, repo, opt.getURLQuery().Encode()), nil, nil, &labels)
}

// GetRepoLabel get one label of repository by repo it
// TODO: maybe we need get a label by name
func (c *Client) GetRepoLabel(owner, repo string, id int64) (*Label, error) {
	label := new(Label)
	return label, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil, label)
}

// CreateLabelOption options for creating a label
type CreateLabelOption struct {
	Name string `json:"name"`
	// example: #00aabb
	Color       string `json:"color"`
	Description string `json:"description"`
}

// CreateLabel create one label of repository
func (c *Client) CreateLabel(owner, repo string, opt CreateLabelOption) (*Label, error) {
	if len(opt.Color) == 6 {
		if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
			opt.Color = "#" + opt.Color
		}
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	label := new(Label)
	return label, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/labels", owner, repo),
		jsonHeader, bytes.NewReader(body), label)
}

// EditLabelOption options for editing a label
type EditLabelOption struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

// EditLabel modify one label with options
func (c *Client) EditLabel(owner, repo string, id int64, opt EditLabelOption) (*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	label := new(Label)
	return label, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), label)
}

// DeleteLabel delete one label of repository by id
// TODO: maybe we need delete by name
func (c *Client) DeleteLabel(owner, repo string, id int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil)
	return err
}

// GetIssueLabels get labels of one issue via issue id
func (c *Client) GetIssueLabels(owner, repo string, index int64, opts ListLabelsOptions) ([]*Label, error) {
	labels := make([]*Label, 0, 5)
	return labels, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/labels?%s", owner, repo, index, opts.getURLQuery().Encode()), nil, nil, &labels)
}

// IssueLabelsOption a collection of labels
type IssueLabelsOption struct {
	// list of label IDs
	Labels []int64 `json:"labels"`
}

// AddIssueLabels add one or more labels to one issue
func (c *Client) AddIssueLabels(owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	var labels []*Label
	return labels, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
}

// ReplaceIssueLabels replace old labels of issue with new labels
func (c *Client) ReplaceIssueLabels(owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	var labels []*Label
	return labels, c.getParsedResponse("PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
}

// DeleteIssueLabel delete one label of one issue by issue id and label id
// TODO: maybe we need delete by label name and issue id
func (c *Client) DeleteIssueLabel(owner, repo string, index, label int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%d", owner, repo, index, label), nil, nil)
	return err
}

// ClearIssueLabels delete all the labels of one issue.
func (c *Client) ClearIssueLabels(owner, repo string, index int64) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), nil, nil)
	return err
}
