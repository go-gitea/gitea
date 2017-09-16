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

// StatusState holds the state of a Status
// It can be "pending", "success", "error", "failure", and "warning"
type StatusState string

const (
	// StatusPending is for when the Status is Pending
	StatusPending StatusState = "pending"
	// StatusSuccess is for when the Status is Success
	StatusSuccess StatusState = "success"
	// StatusError is for when the Status is Error
	StatusError StatusState = "error"
	// StatusFailure is for when the Status is Failure
	StatusFailure StatusState = "failure"
	// StatusWarning is for when the Status is Warning
	StatusWarning StatusState = "warning"
)

// Status holds a single Status of a single Commit
type Status struct {
	ID          int64       `json:"id"`
	State       StatusState `json:"status"`
	TargetURL   string      `json:"target_url"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	Context     string      `json:"context"`
	Creator     *User       `json:"creator"`
	Created     time.Time   `json:"created_at"`
	Updated     time.Time   `json:"updated_at"`
}

// CombinedStatus holds the combined state of several statuses for a single commit
type CombinedStatus struct {
	State      StatusState `json:"state"`
	SHA        string      `json:"sha"`
	TotalCount int         `json:"total_count"`
	Statuses   []*Status   `json:"statuses"`
	Repository *Repository `json:"repository"`
	CommitURL  string      `json:"commit_url"`
	URL        string      `json:"url"`
}

// CreateStatusOption holds the information needed to create a new Status for a Commit
type CreateStatusOption struct {
	State       StatusState `json:"state"`
	TargetURL   string      `json:"target_url"`
	Description string      `json:"description"`
	Context     string      `json:"context"`
}

// ListStatusesOption holds pagination information
type ListStatusesOption struct {
	Page int
}

// CreateStatus creates a new Status for a given Commit
//
// POST /repos/:owner/:repo/statuses/:sha
func (c *Client) CreateStatus(owner, repo, sha string, opts CreateStatusOption) (*Status, error) {
	body, err := json.Marshal(&opts)
	if err != nil {
		return nil, err
	}
	status := &Status{}
	return status, c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/statuses/%s", owner, repo, sha),
		jsonHeader, bytes.NewReader(body), status)
}

// ListStatuses returns all statuses for a given Commit
//
// GET /repos/:owner/:repo/commits/:ref/statuses
func (c *Client) ListStatuses(owner, repo, sha string, opts ListStatusesOption) ([]*Status, error) {
	statuses := make([]*Status, 0, 10)
	return statuses, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/commits/%s/statuses?page=%d", owner, repo, sha, opts.Page), nil, nil, &statuses)
}

// GetCombinedStatus returns the CombinedStatus for a given Commit
//
// GET /repos/:owner/:repo/commits/:ref/status
func (c *Client) GetCombinedStatus(owner, repo, sha string) (*CombinedStatus, error) {
	status := &CombinedStatus{}
	return status, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/commits/%s/status", owner, repo, sha), nil, nil, status)
}
