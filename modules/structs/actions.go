// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs // import "code.gitea.io/gitea/modules/structs"

// ActionRunnerToken represents an action runner token
// swagger:model
type ActionRunnerToken struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}
