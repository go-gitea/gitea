// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

// ScopeType identifies the unit an audit event belongs to (for filtering in UI).
// Target-specific details live in Metadata, not as typed objects in the audit package.
type ScopeType string

const (
	ScopeSystem       ScopeType = "system"
	ScopeUser         ScopeType = "user"
	ScopeOrganization ScopeType = "organization"
	ScopeRepository   ScopeType = "repository"
)
