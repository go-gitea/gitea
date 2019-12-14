// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// CreateUserOption create user options
type CreateUserOption struct {
	SourceID  int64  `json:"source_id"`
	LoginName string `json:"login_name"`
	// required: true
	Username string `json:"username" binding:"Required;AlphaDashDot;MaxSize(40)"`
	FullName string `json:"full_name" binding:"MaxSize(100)"`
	// required: true
	// swagger:strfmt email
	Email string `json:"email" binding:"Required;Email;MaxSize(254)"`
	// required: true
	Password           string `json:"password" binding:"Required;MaxSize(255)"`
	MustChangePassword *bool  `json:"must_change_password"`
	SendNotify         bool   `json:"send_notify"`
}

// EditUserOption edit user options
type EditUserOption struct {
	SourceID  int64  `json:"source_id"`
	LoginName string `json:"login_name"`
	FullName  string `json:"full_name" binding:"MaxSize(100)"`
	// required: true
	// swagger:strfmt email
	Email                   string `json:"email" binding:"Required;Email;MaxSize(254)"`
	Password                string `json:"password" binding:"MaxSize(255)"`
	MustChangePassword      *bool  `json:"must_change_password"`
	Website                 string `json:"website" binding:"MaxSize(50)"`
	Location                string `json:"location" binding:"MaxSize(50)"`
	Active                  *bool  `json:"active"`
	Admin                   *bool  `json:"admin"`
	AllowGitHook            *bool  `json:"allow_git_hook"`
	AllowImportLocal        *bool  `json:"allow_import_local"`
	MaxRepoCreation         *int   `json:"max_repo_creation"`
	ProhibitLogin           *bool  `json:"prohibit_login"`
	AllowCreateOrganization *bool  `json:"allow_create_organization"`
}
