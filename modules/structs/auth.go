// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// swagger:model
type AuthSourceOption struct {
	ID                 int64  `json:"id"`
	AuthenticationName string `json:"authentication_name" binding:"Required"`
	TypeName           string `json:"type_name"`

	IsActive      bool `json:"is_active"`
	IsSyncEnabled bool `json:"is_sync_enabled"`
}
