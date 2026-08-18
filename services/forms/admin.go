// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"gitea.dev/modules/structs"
	"gitea.dev/modules/web/middleware"
)

// AdminCreateUserForm form for admin to create user
type AdminCreateUserForm struct {
	middleware.FormDefaultValidator
	LoginType          string `binding:"Required"`
	LoginName          string
	UserName           string `binding:"Required;Username;MaxSize(40)"`
	Email              string `binding:"Required;Email;MaxSize(254)"`
	Password           string `binding:"MaxSize(255)"`
	SendNotify         bool
	MustChangePassword bool
	Visibility         structs.VisibleType
}

// AdminCreateBadgeForm form for admin to create badge
type AdminCreateBadgeForm struct {
	middleware.FormDefaultValidator
	Slug        string `binding:"Required;BadgeSlug" locale:"admin.badges.slug"`
	Description string `binding:"Required" locale:"admin.badges.description"`
	ImageURL    string `binding:"ValidUrl" locale:"admin.badges.image_url"`
}

// AdminEditBadgeForm form for admin to edit badge
type AdminEditBadgeForm struct {
	middleware.FormDefaultValidator
	Description string `binding:"Required" locale:"admin.badges.description"`
	ImageURL    string `binding:"ValidUrl" locale:"admin.badges.image_url"`
}

// AdminEditUserForm form for admin to create user
type AdminEditUserForm struct {
	middleware.FormDefaultValidator
	LoginType               string `binding:"Required"`
	UserName                string `binding:"Username;MaxSize(40)"`
	LoginName               string
	FullName                string `binding:"MaxSize(100)"`
	Email                   string `binding:"Required;Email;MaxSize(254)"`
	Password                string `binding:"MaxSize(255)"`
	Website                 string `binding:"ValidUrl;MaxSize(255)"`
	Location                string `binding:"MaxSize(50)"`
	Language                string `binding:"MaxSize(5)"`
	MaxRepoCreation         int
	Active                  bool
	Admin                   bool
	Restricted              bool
	AllowGitHook            bool
	AllowImportLocal        bool
	AllowCreateOrganization bool
	ProhibitLogin           bool
	Reset2FA                bool `form:"reset_2fa"`
	Visibility              structs.VisibleType
}

// AdminDashboardForm form for admin dashboard operations
type AdminDashboardForm struct {
	middleware.FormDefaultValidator
	Op   string `binding:"required"`
	From string
}
