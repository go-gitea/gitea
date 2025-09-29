// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// CreateUserOption create user options
type CreateUserOption struct {
	// The authentication source ID to associate with the user
	SourceID int64 `json:"source_id"`
	// identifier of the user, provided by the external authenticator (if configured)
	// default: empty
	LoginName string `json:"login_name"`
	// username of the user
	// required: true
	Username string `json:"username" binding:"Required;Username;MaxSize(40)"`
	// The full display name of the user
	FullName string `json:"full_name" binding:"MaxSize(100)"`
	// required: true
	// swagger:strfmt email
	Email string `json:"email" binding:"Required;Email;MaxSize(254)"`
	// The plain text password for the user
	Password string `json:"password" binding:"MaxSize(255)"`
	// Whether the user must change password on first login
	MustChangePassword *bool `json:"must_change_password"`
	// Whether to send welcome notification email to the user
	SendNotify bool `json:"send_notify"`
	// Whether the user has restricted access privileges
	Restricted *bool `json:"restricted"`
	// User visibility level: public, limited, or private
	Visibility string `json:"visibility" binding:"In(,public,limited,private)"`

	// For explicitly setting the user creation timestamp. Useful when users are
	// migrated from other systems. When omitted, the user's creation timestamp
	// will be set to "now".
	Created *time.Time `json:"created_at"`
}

// EditUserOption edit user options
type EditUserOption struct {
	// required: true
	// The authentication source ID to associate with the user
	SourceID int64 `json:"source_id"`
	// identifier of the user, provided by the external authenticator (if configured)
	// default: empty
	// required: true
	LoginName string `json:"login_name" binding:"Required"`
	// swagger:strfmt email
	// The email address of the user
	Email *string `json:"email" binding:"MaxSize(254)"`
	// The full display name of the user
	FullName *string `json:"full_name" binding:"MaxSize(100)"`
	// The plain text password for the user
	Password string `json:"password" binding:"MaxSize(255)"`
	// Whether the user must change password on next login
	MustChangePassword *bool `json:"must_change_password"`
	// The user's personal website URL
	Website *string `json:"website" binding:"OmitEmpty;ValidUrl;MaxSize(255)"`
	// The user's location or address
	Location *string `json:"location" binding:"MaxSize(50)"`
	// The user's personal description or bio
	Description *string `json:"description" binding:"MaxSize(255)"`
	// Whether the user account is active
	Active *bool `json:"active"`
	// Whether the user has administrator privileges
	Admin *bool `json:"admin"`
	// Whether the user can use Git hooks
	AllowGitHook *bool `json:"allow_git_hook"`
	// Whether the user can import local repositories
	AllowImportLocal *bool `json:"allow_import_local"`
	// Maximum number of repositories the user can create
	MaxRepoCreation *int `json:"max_repo_creation"`
	// Whether the user is prohibited from logging in
	ProhibitLogin *bool `json:"prohibit_login"`
	// Whether the user can create organizations
	AllowCreateOrganization *bool `json:"allow_create_organization"`
	// Whether the user has restricted access privileges
	Restricted *bool `json:"restricted"`
	// User visibility level: public, limited, or private
	Visibility string `json:"visibility" binding:"In(,public,limited,private)"`
}
