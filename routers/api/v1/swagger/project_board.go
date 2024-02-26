// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// ProjectBoard
// swagger:response ProjectBoard
type swaggerProjectBoard struct {
	// in:body
	Body api.ProjectBoard `json:"body"`
}

// ProjectBoardList
// swagger:response ProjectBoardList
type swaggerProjectBoardList struct {
	// in:body
	Body []api.ProjectBoard `json:"body"`
}
