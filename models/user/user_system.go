// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strings"

	"code.gitea.io/gitea/modules/structs"
)

const (
	GhostUserID   int64 = -1
	GhostUserName       = "Ghost"
)

// NewGhostUser creates and returns a fake user for someone has deleted their account.
func NewGhostUser() *User {
	return &User{
		ID:        GhostUserID,
		Name:      GhostUserName,
		LowerName: strings.ToLower(GhostUserName),
	}
}

func IsGhostUserName(name string) bool {
	return strings.EqualFold(name, GhostUserName)
}

// IsGhost check if user is fake user for a deleted account
func (u *User) IsGhost() bool {
	if u == nil {
		return false
	}
	return u.ID == GhostUserID && u.Name == GhostUserName
}

const (
	ActionsUserID    int64 = -2
	ActionsUserName        = "gitea-actions"
	ActionsUserEmail       = "teabot@gitea.io"
)

func IsGiteaActionsUserName(name string) bool {
	return strings.EqualFold(name, ActionsUserName)
}

// NewActionsUser creates and returns a fake user for running the actions.
func NewActionsUser() *User {
	return &User{
		ID:               ActionsUserID,
		Name:             ActionsUserName,
		LowerName:        ActionsUserName,
		IsActive:         true,
		FullName:         "Gitea Actions",
		Email:            ActionsUserEmail,
		KeepEmailPrivate: true,
		LoginName:        ActionsUserName,
		Type:             UserTypeBot,
		Visibility:       structs.VisibleTypePublic,
	}
}

func (u *User) IsGiteaActions() bool {
	return u != nil && u.ID == ActionsUserID
}

const (
	ProjectWorkflowsUserID    int64 = -3
	ProjectWorkflowsUserName        = "project-workflows"
	ProjectWorkflowsUserEmail       = "workflows@gitea.io"
)

func IsGiteaWorkflowsUserName(name string) bool {
	return strings.EqualFold(name, ProjectWorkflowsUserName)
}

// NewProjectWorkflowsUser creates and returns a fake user for running the project workflows.
func NewProjectWorkflowsUser() *User {
	return &User{
		ID:                      ProjectWorkflowsUserID,
		Name:                    ProjectWorkflowsUserName,
		LowerName:               ProjectWorkflowsUserName,
		IsActive:                true,
		FullName:                "Project Workflows",
		Email:                   ProjectWorkflowsUserEmail,
		KeepEmailPrivate:        true,
		LoginName:               ProjectWorkflowsUserName,
		Type:                    UserTypeBot,
		AllowCreateOrganization: true,
		Visibility:              structs.VisibleTypePublic,
	}
}

func (u *User) IsProjectWorkflows() bool {
	return u != nil && u.ID == ProjectWorkflowsUserID
}

func GetSystemUserByName(name string) *User {
	if IsGhostUserName(name) {
		return NewGhostUser()
	}
	if IsGiteaActionsUserName(name) {
		return NewActionsUser()
	}
	if IsGiteaWorkflowsUserName(name) {
		return NewProjectWorkflowsUser()
	}
	return nil
}
