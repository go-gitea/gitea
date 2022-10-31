// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import user_model "code.gitea.io/gitea/models/user"

// NewBotUser creates and returns a fake user for running the build.
func NewBotUser() *user_model.User {
	return &user_model.User{
		ID:        -2,
		Name:      "gitea-bots",
		LowerName: "gitea-bots",
	}
}
