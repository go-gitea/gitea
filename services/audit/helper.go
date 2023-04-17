// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	user_model "code.gitea.io/gitea/models/user"
)

func NewCLIUser() *user_model.User {
	return &user_model.User{
		ID:        -1,
		Name:      "CLI",
		LowerName: "cli",
	}
}

func NewAuthenticationSourceUser() *user_model.User {
	return &user_model.User{
		ID:        -1,
		Name:      "AuthenticationSource",
		LowerName: "authenticationsource",
	}
}

func UserActiveString(isActive bool) string {
	if isActive {
		return "active"
	}
	return "inactive"
}

func UserAdminString(isAdmin bool) string {
	if isAdmin {
		return "admin"
	}
	return "normal user"
}

func UserRestrictedString(isRestricted bool) string {
	if isRestricted {
		return "restricted"
	}
	return "unrestricted"
}

func PublicString(isPublic bool) string {
	if isPublic {
		return "public"
	}
	return "private"
}
