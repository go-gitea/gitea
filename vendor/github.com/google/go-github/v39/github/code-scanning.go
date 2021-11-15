// Copyright 2020 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CodeScanningService handles communication with the code scanning related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/code-scanning/
type CodeScanningService service

// Rule represents the complete details of GitHub Code Scanning alert type.
type Rule struct {
	ID                    *string  `json:"id,omitempty"`
	Severity              *string  `json:"severity,omitempty"`
	Description           *string  `json:"description,omitempty"`
	Name                  *string  `json:"name,omitempty"`
	SecuritySeverityLevel *string  `json:"security_severity_level,omitempty"`
	FullDescription       *string  `json:"full_description,omitempty"`
	Tags                  []string `json:"tags,omitempty"`
	Help                  *string  `json:"help,omitempty"`
}

// Location represents the exact location of the GitHub Code Scanning Alert in the scanned project.
type Location struct {
	Path        *string `json:"path,omitempty"`
	StartLine   *int    `json:"start_line,omitempty"`
	EndLine     *int    `json:"end_line,omitempty"`
	StartColumn *int    `json:"start_column,omitempty"`
	EndColumn   *int    `json:"end_column,omitempty"`
}

// Message is a part of MostRecentInstance struct which provides the appropriate message when any action is performed on the analysis object.
type Message struct {
	Text *string `json:"text,omitempty"`
}

// MostRecentInstance provides details of the most recent instance of this alert for the default branch or for the specified Git reference.
type MostRecentInstance struct {
	Ref             *string   `json:"ref,omitempty"`
	AnalysisKey     *string   `json:"analysis_key,omitempty"`
	Environment     *string   `json:"environment,omitempty"`
	State           *string   `json:"state,omitempty"`
	CommitSHA       *string   `json:"commit_sha,omitempty"`
	Message         *Message  `json:"message,omitempty"`
	Location        *Location `json:"location,omitempty"`
	Classifications []string  `json:"classifications,omitempty"`
}

// Tool represents the tool used to generate a GitHub Code Scanning Alert.
type Tool struct {
	Name    *string `json:"name,omitempty"`
	GUID    *string `json:"guid,omitempty"`
	Version *string `json:"version,omitempty"`
}

// Alert represents an individual GitHub Code Scanning Alert on a single repository.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/code-scanning#list-code-scanning-alerts-for-a-repository
type Alert struct {
	RuleID             *string             `json:"rule_id,omitempty"`
	RuleSeverity       *string             `json:"rule_severity,omitempty"`
	RuleDescription    *string             `json:"rule_description,omitempty"`
	Rule               *Rule               `json:"rule,omitempty"`
	Tool               *Tool               `json:"tool,omitempty"`
	CreatedAt          *Timestamp          `json:"created_at,omitempty"`
	State              *string             `json:"state,omitempty"`
	ClosedBy           *User               `json:"closed_by,omitempty"`
	ClosedAt           *Timestamp          `json:"closed_at,omitempty"`
	URL                *string             `json:"url,omitempty"`
	HTMLURL            *string             `json:"html_url,omitempty"`
	MostRecentInstance *MostRecentInstance `json:"most_recent_instance,omitempty"`
	DismissedBy        *User               `json:"dismissed_by,omitempty"`
	DismissedAt        *Timestamp          `json:"dismissed_at,omitempty"`
	DismissedReason    *string             `json:"dismissed_reason,omitempty"`
	InstancesURL       *string             `json:"instances_url,omitempty"`
}

// ID returns the ID associated with an alert. It is the number at the end of the security alert's URL.
func (a *Alert) ID() int64 {
	if a == nil {
		return 0
	}

	s := a.GetHTMLURL()

	// Check for an ID to parse at the end of the url
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}

	// Return the alert ID as a 64-bit integer. Unable to convert or out of range returns 0.
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return id
}

// AlertListOptions specifies optional parameters to the CodeScanningService.ListAlerts
// method.
type AlertListOptions struct {
	// State of the code scanning alerts to list. Set to closed to list only closed code scanning alerts. Default: open
	State string `url:"state,omitempty"`

	// Return code scanning alerts for a specific branch reference. The ref must be formatted as heads/<branch name>.
	Ref string `url:"ref,omitempty"`

	ListOptions
}

// ListAlertsForRepo lists code scanning alerts for a repository.
//
// Lists all open code scanning alerts for the default branch (usually master) and protected branches in a repository.
// You must use an access token with the security_events scope to use this endpoint. GitHub Apps must have the security_events
// read permission to use this endpoint.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/code-scanning/#list-code-scanning-alerts-for-a-repository
func (s *CodeScanningService) ListAlertsForRepo(ctx context.Context, owner, repo string, opts *AlertListOptions) ([]*Alert, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-scanning/alerts", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var alerts []*Alert
	resp, err := s.client.Do(ctx, req, &alerts)
	if err != nil {
		return nil, resp, err
	}

	return alerts, resp, nil
}

// GetAlert gets a single code scanning alert for a repository.
//
// You must use an access token with the security_events scope to use this endpoint.
// GitHub Apps must have the security_events read permission to use this endpoint.
//
// The security alert_id is the number at the end of the security alert's URL.
//
// GitHub API docs: https://docs.github.com/en/free-pro-team@latest/rest/reference/code-scanning/#get-a-code-scanning-alert
func (s *CodeScanningService) GetAlert(ctx context.Context, owner, repo string, id int64) (*Alert, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-scanning/alerts/%v", owner, repo, id)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	a := new(Alert)
	resp, err := s.client.Do(ctx, req, a)
	if err != nil {
		return nil, resp, err
	}

	return a, resp, nil
}
