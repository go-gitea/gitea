// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// GetAuditLogOptions sets up optional parameters to query audit-log endpoint.
type GetAuditLogOptions struct {
	Phrase  *string `json:"phrase,omitempty"`  // A search phrase. (Optional.)
	Include *string `json:"include,omitempty"` // Event type includes. Can be one of "web", "git", "all". Default: "web". (Optional.)
	Order   *string `json:"order,omitempty"`   // The order of audit log events. Can be one of "asc" or "desc". Default: "desc". (Optional.)

	ListCursorOptions
}

// HookConfig describes metadata about a webhook configuration.
type HookConfig struct {
	ContentType *string `json:"content_type,omitempty"`
	InsecureSSL *string `json:"insecure_ssl,omitempty"`
	URL         *string `json:"url,omitempty"`

	// Secret is returned obfuscated by GitHub, but it can be set for outgoing requests.
	Secret *string `json:"secret,omitempty"`
}

// AuditEntry describes the fields that may be represented by various audit-log "action" entries.
// For a list of actions see - https://docs.github.com/en/github/setting-up-and-managing-organizations-and-teams/reviewing-the-audit-log-for-your-organization#audit-log-actions
type AuditEntry struct {
	Action                *string     `json:"action,omitempty"` // The name of the action that was performed, for example `user.login` or `repo.create`.
	Active                *bool       `json:"active,omitempty"`
	ActiveWas             *bool       `json:"active_was,omitempty"`
	Actor                 *string     `json:"actor,omitempty"` // The actor who performed the action.
	BlockedUser           *string     `json:"blocked_user,omitempty"`
	Business              *string     `json:"business,omitempty"`
	CancelledAt           *Timestamp  `json:"cancelled_at,omitempty"`
	CompletedAt           *Timestamp  `json:"completed_at,omitempty"`
	Conclusion            *string     `json:"conclusion,omitempty"`
	Config                *HookConfig `json:"config,omitempty"`
	ConfigWas             *HookConfig `json:"config_was,omitempty"`
	ContentType           *string     `json:"content_type,omitempty"`
	CreatedAt             *Timestamp  `json:"created_at,omitempty"`
	DeployKeyFingerprint  *string     `json:"deploy_key_fingerprint,omitempty"`
	DocumentID            *string     `json:"_document_id,omitempty"`
	Emoji                 *string     `json:"emoji,omitempty"`
	EnvironmentName       *string     `json:"environment_name,omitempty"`
	Event                 *string     `json:"event,omitempty"`
	Events                []string    `json:"events,omitempty"`
	EventsWere            []string    `json:"events_were,omitempty"`
	Explanation           *string     `json:"explanation,omitempty"`
	Fingerprint           *string     `json:"fingerprint,omitempty"`
	HeadBranch            *string     `json:"head_branch,omitempty"`
	HeadSHA               *string     `json:"head_sha,omitempty"`
	HookID                *int64      `json:"hook_id,omitempty"`
	IsHostedRunner        *bool       `json:"is_hosted_runner,omitempty"`
	JobName               *string     `json:"job_name,omitempty"`
	LimitedAvailability   *bool       `json:"limited_availability,omitempty"`
	Message               *string     `json:"message,omitempty"`
	Name                  *string     `json:"name,omitempty"`
	OldUser               *string     `json:"old_user,omitempty"`
	OpenSSHPublicKey      *string     `json:"openssh_public_key,omitempty"`
	Org                   *string     `json:"org,omitempty"`
	PreviousVisibility    *string     `json:"previous_visibility,omitempty"`
	ReadOnly              *string     `json:"read_only,omitempty"`
	Repo                  *string     `json:"repo,omitempty"`
	Repository            *string     `json:"repository,omitempty"`
	RepositoryPublic      *bool       `json:"repository_public,omitempty"`
	RunnerGroupID         *string     `json:"runner_group_id,omitempty"`
	RunnerGroupName       *string     `json:"runner_group_name,omitempty"`
	RunnerID              *string     `json:"runner_id,omitempty"`
	RunnerLabels          []string    `json:"runner_labels,omitempty"`
	RunnerName            *string     `json:"runner_name,omitempty"`
	SecretsPassed         []string    `json:"secrets_passed,omitempty"`
	SourceVersion         *string     `json:"source_version,omitempty"`
	StartedAt             *Timestamp  `json:"started_at,omitempty"`
	TargetLogin           *string     `json:"target_login,omitempty"`
	TargetVersion         *string     `json:"target_version,omitempty"`
	Team                  *string     `json:"team,omitempty"`
	Timestamp             *Timestamp  `json:"@timestamp,omitempty"`              // The time the audit log event occurred, given as a [Unix timestamp](http://en.wikipedia.org/wiki/Unix_time).
	TransportProtocolName *string     `json:"transport_protocol_name,omitempty"` // A human readable name for the protocol (for example, HTTP or SSH) used to transfer Git data.
	TransportProtocol     *int        `json:"transport_protocol,omitempty"`      // The type of protocol (for example, HTTP=1 or SSH=2) used to transfer Git data.
	TriggerID             *int64      `json:"trigger_id,omitempty"`
	User                  *string     `json:"user,omitempty"`       // The user that was affected by the action performed (if available).
	Visibility            *string     `json:"visibility,omitempty"` // The repository visibility, for example `public` or `private`.
	WorkflowID            *int64      `json:"workflow_id,omitempty"`
	WorkflowRunID         *int64      `json:"workflow_run_id,omitempty"`
}

// GetAuditLog gets the audit-log entries for an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/orgs#get-the-audit-log-for-an-organization
func (s *OrganizationsService) GetAuditLog(ctx context.Context, org string, opts *GetAuditLogOptions) ([]*AuditEntry, *Response, error) {
	u := fmt.Sprintf("orgs/%v/audit-log", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var auditEntries []*AuditEntry
	resp, err := s.client.Do(ctx, req, &auditEntries)
	if err != nil {
		return nil, resp, err
	}

	return auditEntries, resp, nil
}
