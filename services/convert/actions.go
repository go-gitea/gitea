// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	actions_model "code.gitea.io/gitea/models/actions"
	api "code.gitea.io/gitea/modules/structs"
)

func ToAPIActionRunnerToken(token *actions_model.ActionRunnerToken) *api.ActionRunnerToken {
	return &api.ActionRunnerToken{
		ID:    token.ID,
		Token: token.Token,
	}
}
