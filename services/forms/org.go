// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"gitea.dev/modules/structs"
	"gitea.dev/modules/web/middleware"
)

// ________                            .__                __  .__
// \_____  \_______  _________    ____ |__|____________ _/  |_|__| ____   ____
//  /   |   \_  __ \/ ___\__  \  /    \|  \___   /\__  \\   __\  |/  _ \ /    \
// /    |    \  | \/ /_/  > __ \|   |  \  |/    /  / __ \|  | |  (  <_> )   |  \
// \_______  /__|  \___  (____  /___|  /__/_____ \(____  /__| |__|\____/|___|  /
//         \/     /_____/     \/     \/         \/     \/                    \/

// CreateOrgForm form for creating organization
type CreateOrgForm struct {
	middleware.FormDefaultValidator
	OrgName                   string `binding:"Required;Username;MaxSize(40)" locale:"org.org_name_holder"`
	Visibility                structs.VisibleType
	RepoAdminChangeTeamAccess bool
}

// UpdateOrgSettingForm form for updating organization settings
type UpdateOrgSettingForm struct {
	middleware.FormDefaultValidator
	FullName                  *string `binding:"MaxSize(100)"`
	Email                     *string `binding:"MaxSize(255)"`
	Description               *string `binding:"MaxSize(255)"`
	Website                   *string `binding:"ValidUrl;MaxSize(255)"`
	Location                  *string `binding:"MaxSize(50)"`
	MaxRepoCreation           *int
	RepoAdminChangeTeamAccess *bool
}

type RenameOrgForm struct {
	middleware.FormDefaultValidator
	OrgName    string `binding:"Required"`
	NewOrgName string `binding:"Required;Username;MaxSize(40)" locale:"org.org_name_holder"`
}

// ___________
// \__    ___/___ _____    _____
//   |    |_/ __ \\__  \  /     \
//   |    |\  ___/ / __ \|  Y Y  \
//   |____| \___  >____  /__|_|  /
//              \/     \/      \/

// CreateTeamForm form for creating team
type CreateTeamForm struct {
	middleware.FormDefaultValidator
	TeamName         string `binding:"Required;AlphaDashDot;MaxSize(255)"`
	Description      string `binding:"MaxSize(255)"`
	Permission       string
	RepoAccess       string
	CanCreateOrgRepo bool
	Visibility       string `binding:"OmitEmpty;In(public,limited,private)"`
}
