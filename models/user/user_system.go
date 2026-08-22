// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strconv"
	"strings"

	"gitea.dev/modules/structs"
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

// withSystemUserRefID marks a system user as acting for one credential.
// LoginName is for only internal usage in this case, so it can be moved to other fields in the future.
func withSystemUserRefID(u *User, id int64) *User {
	u.LoginSource = -1
	u.LoginName = "@" + u.Name + "/" + strconv.FormatInt(id, 10)
	return u
}

func systemUserRefID(u *User, systemUserID int64, name string) (int64, bool) {
	if u == nil || u.ID != systemUserID {
		return 0, false
	}
	prefix, payload, _ := strings.Cut(u.LoginName, "/")
	if prefix != "@"+name {
		return 0, false
	} else if id, err := strconv.ParseInt(payload, 10, 64); err == nil {
		return id, true
	}
	return 0, false
}

func NewActionsUserWithTaskID(id int64) *User {
	return withSystemUserRefID(NewActionsUser(), id)
}

func GetActionsUserTaskID(u *User) (int64, bool) {
	return systemUserRefID(u, ActionsUserID, ActionsUserName)
}

func (u *User) IsGiteaActions() bool {
	return u != nil && u.ID == ActionsUserID
}

const (
	DeployKeyUserID   int64 = -3
	DeployKeyUserName       = "gitea-deploy-key"
)

// NewDeployKeyUser creates and returns a fake user for a request authenticated by a deploy key.
// It is never the owner of anything, it only carries the key whose permissions the request gets.
func NewDeployKeyUser() *User {
	return &User{
		ID:         DeployKeyUserID,
		Name:       DeployKeyUserName,
		LowerName:  DeployKeyUserName,
		IsActive:   true,
		FullName:   "Gitea Deploy Key",
		LoginName:  DeployKeyUserName,
		Type:       UserTypeBot,
		Visibility: structs.VisibleTypePublic,
	}
}

func NewDeployKeyUserWithKeyID(id int64) *User {
	return withSystemUserRefID(NewDeployKeyUser(), id)
}

func GetDeployKeyUserKeyID(u *User) (int64, bool) {
	return systemUserRefID(u, DeployKeyUserID, DeployKeyUserName)
}

func GetSystemUserByName(name string) *User {
	for _, newFunc := range globalVars().systemUserNewFuncs {
		if u := newFunc(); strings.EqualFold(name, u.Name) {
			return u
		}
	}
	return nil
}
