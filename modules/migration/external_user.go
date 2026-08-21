// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// ExternalUser identifies a user in the source forge.
type ExternalUser struct {
	ID   int64  `yaml:"id" json:"id"`
	Name string `yaml:"name" json:"name"`
}

// GetExternalName implements user_model.ExternalUserMigrated.
func (u *ExternalUser) GetExternalName() string { return u.Name }

// GetExternalID implements user_model.ExternalUserMigrated.
func (u *ExternalUser) GetExternalID() int64 { return u.ID }
