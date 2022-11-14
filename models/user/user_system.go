// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"strings"

	"code.gitea.io/gitea/modules/structs"
)

// NewGhostUser creates and returns a fake user for someone has deleted their account.
func NewGhostUser() *User {
	return &User{
		ID:        -1,
		Name:      "Ghost",
		LowerName: "ghost",
	}
}

// IsGhost check if user is fake user for a deleted account
func (u *User) IsGhost() bool {
	if u == nil {
		return false
	}
	return u.ID == -1 && u.Name == "Ghost"
}

// NewReplaceUser creates and returns a fake user for external user
func NewReplaceUser(name string) *User {
	return &User{
		ID:        -1,
		Name:      name,
		LowerName: strings.ToLower(name),
	}
}

const BotUserID = -2

// NewBotUser creates and returns a fake user for running the build.
func NewBotUser() *User {
	return &User{
		ID:                      BotUserID,
		Name:                    "gitea-bots",
		LowerName:               "gitea-bots",
		IsActive:                true,
		FullName:                "Gitea Bots",
		Email:                   "teabot@gitea.io",
		KeepEmailPrivate:        true,
		LoginName:               "gitea-bots",
		Type:                    UserTypeIndividual,
		AllowCreateOrganization: true,
		Visibility:              structs.VisibleTypePublic,
	}
}

func (u *User) IsBots() bool {
	if u == nil {
		return false
	}
	return u.ID == BotUserID && u.Name == "gitea-bots"
}
