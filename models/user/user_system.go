// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
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

// newSystemUser creates and returns a fake user for system use.
// The builtin username can be wrapped in parentheses to avoid conflicts with real usernames.
func newSystemUser(id int64, name, fullName string) *User {
	return &User{
		ID:         id,
		Name:       name,
		LowerName:  strings.ToLower(name),
		IsActive:   true,
		FullName:   fullName,
		Type:       UserTypeBot,
		Visibility: structs.VisibleTypePublic,
	}
}

const (
	ActionsUserID   int64 = -2
	DeployKeyUserID int64 = -3
)

// NewActionsUser creates and returns a fake user for running the actions.
func NewActionsUser() *User {
	return newSystemUser(ActionsUserID, "gitea-actions", "Gitea Actions")
}

func GetActionsUserTaskID(u *User) (int64, bool) {
	if u == nil || u.ExtDoerData == nil || u.ID != ActionsUserID {
		return 0, false
	}
	extData := u.ExtDoerData.(*extDoerGiteaActions) //nolint:forcetypeassert // must be valid
	return extData.TaskID, true
}

func NewActionsUserWithTaskID(id int64) *User {
	u := NewActionsUser()
	u.ExtDoerData = &extDoerGiteaActions{TaskID: id}
	return u
}

func NewDeployKeyUser() *User {
	return newSystemUser(DeployKeyUserID, "(deploy-key)", "Deploy Key")
}

func GetDeployKeyUserDeployKeyID(u *User) (int64, bool) {
	// ok, the function name seems wordy, it is intentionally to distinguish from other "keys" like "public key id"
	// it was a mess in the "pre-receive" hook code
	if u == nil || u.ExtDoerData == nil || u.ID != DeployKeyUserID {
		return 0, false
	}
	extData := u.ExtDoerData.(*extDoerDeployKey) //nolint:forcetypeassert // must be valid
	return extData.DeployKeyID, true
}

func NewDeployKeyUserWithKeyID(id int64) *User {
	u := NewDeployKeyUser()
	u.ExtDoerData = &extDoerDeployKey{DeployKeyID: id}
	return u
}

const (
	CLIUserID   int64 = -4
	CLIUserName       = "CLI"
)

func NewCLIUser() *User {
	return &User{
		ID:        CLIUserID,
		Name:      CLIUserName,
		LowerName: strings.ToLower(CLIUserName),
	}
}

const (
	AuthenticationSourceUserID   int64 = -5
	AuthenticationSourceUserName       = "AuthenticationSource"
)

func NewAuthenticationSourceUser() *User {
	return &User{
		ID:        AuthenticationSourceUserID,
		Name:      AuthenticationSourceUserName,
		LowerName: strings.ToLower(AuthenticationSourceUserName),
	}
}

func GetSystemUserByName(name string) *User {
	lowerName := strings.ToLower(name)
	uid := globalVars().systemUserNameIdMap[lowerName]
	if fn := globalVars().systemUserNewFuncs[uid]; fn != nil {
		return fn()
	}
	return nil
}

func GetDoerUser(ctx context.Context, id int64, extDoerData string) (u *User, _ error) {
	if id > 0 {
		return GetUserByID(ctx, id)
	}
	switch id {
	case ActionsUserID:
		u = NewActionsUser()
		u.ExtDoerData = &extDoerGiteaActions{}
	case DeployKeyUserID:
		u = NewDeployKeyUser()
		u.ExtDoerData = &extDoerDeployKey{}
	default:
		return nil, ErrUserNotExist{UID: id}
	}
	return u, u.ExtDoerData.DecodeFromString(extDoerData)
}
