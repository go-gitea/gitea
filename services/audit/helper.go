// Copyright 2024 The Gitea Authors. All rights reserved.
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
