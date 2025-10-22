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
	WorkflowsUserID    int64 = -3
	WorkflowsUserName        = "gitea-workflows"
	WorkflowsUserEmail       = "workflows@gitea.io"
)

func IsGiteaWorkflowsUserName(name string) bool {
	return strings.EqualFold(name, WorkflowsUserName)
}

// NewWorkflowsUser creates and returns a fake user for running the workflows.
func NewWorkflowsUser() *User {
	return &User{
		ID:                      WorkflowsUserID,
		Name:                    WorkflowsUserName,
		LowerName:               WorkflowsUserName,
		IsActive:                true,
		FullName:                "Gitea Workflows",
		Email:                   WorkflowsUserEmail,
		KeepEmailPrivate:        true,
		LoginName:               WorkflowsUserName,
		Type:                    UserTypeBot,
		AllowCreateOrganization: true,
		Visibility:              structs.VisibleTypePublic,
	}
}

func (u *User) IsGiteaWorkflows() bool {
	return u != nil && u.ID == WorkflowsUserID
}

func GetSystemUserByName(name string) *User {
	if IsGhostUserName(name) {
		return NewGhostUser()
	}
	if IsGiteaActionsUserName(name) {
		return NewActionsUser()
	}
	if IsGiteaWorkflowsUserName(name) {
		return NewWorkflowsUser()
	}
	return nil
}
