// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// Project
// swagger:response Project
type swaggerProject struct {
	// in:body
	Body api.Project `json:"body"`
}

// ProjectList
// swagger:response ProjectList
type swaggerProjectList struct {
	// in:body
	Body []api.Project `json:"body"`
}

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
