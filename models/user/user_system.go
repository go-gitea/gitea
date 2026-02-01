// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strconv"
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

func NewActionsUserWithTaskID(id int64) *User {
	u := NewActionsUser()
	// LoginName is for only internal usage in this case, so it can be moved to other fields in the future
	u.LoginSource = -1
	u.LoginName = "@" + ActionsUserName + "/" + strconv.FormatInt(id, 10)
	return u
}

func GetActionsUserTaskID(u *User) (int64, bool) {
	if u == nil || u.ID != ActionsUserID {
		return 0, false
	}
	prefix, payload, _ := strings.Cut(u.LoginName, "/")
	if prefix != "@"+ActionsUserName {
		return 0, false
	} else if taskID, err := strconv.ParseInt(payload, 10, 64); err == nil {
		return taskID, true
	}
	return 0, false
}

func (u *User) IsGiteaActions() bool {
	return u != nil && u.ID == ActionsUserID
}

func GetSystemUserByName(name string) *User {
	if strings.EqualFold(name, GhostUserName) {
		return NewGhostUser()
	}
	if strings.EqualFold(name, ActionsUserName) {
		return NewActionsUser()
	}
	return nil
}
