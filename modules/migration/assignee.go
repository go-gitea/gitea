// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

type Assignee struct {
	UserID   int64  `yaml:"user_id" json:"user_id"`
	UserName string `yaml:"user_name" json:"user_name"`
}

// GetExternalName ExternalUserMigrated interface
func (r *Assignee) GetExternalName() string { return r.UserName }

// GetExternalID ExternalUserMigrated interface
func (r *Assignee) GetExternalID() int64 { return r.UserID }
