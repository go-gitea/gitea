// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strings"

	"code.gitea.io/gitea/modules/structs"
)

const (
	GhostUserID        = -1
	GhostUserName      = "Ghost"
	GhostUserLowerName = "ghost"
)

// NewGhostUser creates and returns a fake user for someone has deleted their account.
func NewGhostUser() *User {
	return &User{
		ID:        GhostUserID,
		Name:      GhostUserName,
		LowerName: GhostUserLowerName,
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

// NewReplaceUser creates and returns a fake user for external user
func NewReplaceUser(name string) *User {
	return &User{
		ID:        0,
		Name:      name,
		LowerName: strings.ToLower(name),
	}
}

const (
	ActionsUserID   = -2
	ActionsUserName = "gitea-actions"
	ActionsFullName = "Gitea Actions"
	ActionsEmail    = "teabot@gitea.io"
)

func IsGiteaActionsUserName(name string) bool {
	return strings.EqualFold(name, ActionsUserName)
}

// NewActionsUser creates and returns a fake user for running the actions.
func NewActionsUser() *User {
	return &User{
		ID:                      ActionsUserID,
		Name:                    ActionsUserName,
		LowerName:               ActionsUserName,
		IsActive:                true,
		FullName:                ActionsFullName,
		Email:                   ActionsEmail,
		KeepEmailPrivate:        true,
		LoginName:               ActionsUserName,
		Type:                    UserTypeIndividual,
		AllowCreateOrganization: true,
		Visibility:              structs.VisibleTypePublic,
	}
}

func (u *User) IsGiteaActions() bool {
	return u != nil && u.ID == ActionsUserID
}

func GetSystemUserByName(name string) *User {
	if IsGhostUserName(name) {
		return NewGhostUser()
	}
	if IsGiteaActionsUserName(name) {
		return NewActionsUser()
	}
	return nil
}
