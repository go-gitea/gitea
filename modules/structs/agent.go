// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// AgentEnrollOption describes a request to create an agent account.
type AgentEnrollOption struct {
	// required: true
	// Username can be raw machine identity like "whoami@hostname"; server normalizes it.
	Username string `json:"username" binding:"Required;MaxSize(255)"`
	// Optional human-contact email. If omitted, the server generates a placeholder address.
	Email string `json:"email" binding:"OmitEmpty;MaxSize(254)"`
	// Optional machine identity metadata (for example "whoami@hostname").
	MachineIdentity string `json:"machine_identity" binding:"MaxSize(255)"`
	// Optional network identifier metadata (for example "10.10.0.12").
	NetworkIdentifier string `json:"network_identifier" binding:"MaxSize(255)"`
	FullName          string `json:"full_name" binding:"MaxSize(100)"`
	// OwnerAgent allows this agent to invite other agents.
	OwnerAgent bool `json:"owner_agent"`
	// TokenName is the personal access token label. Defaults to "agent-enroll-token".
	TokenName string `json:"token_name" binding:"MaxSize(255)"`
	// TokenScopes defaults to ["write:repository","write:user"] when auto bootstrap repo is enabled,
	// otherwise to ["public-only","read:repository"].
	TokenScopes []string `json:"token_scopes"`
}

// AgentEnrollResponse returns the created agent and bootstrap token.
type AgentEnrollResponse struct {
	User         *User    `json:"user"`
	Token        string   `json:"token"`
	TokenName    string   `json:"token_name"`
	TokenScopes  []string `json:"token_scopes"`
	IsOwnerAgent bool     `json:"is_owner_agent"`
}
