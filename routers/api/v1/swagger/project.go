// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// Project
// swagger:response Project
type swaggerResponseProject struct {
	// in:body
	Body api.Project `json:"body"`
}

// ProjectList
// swagger:response ProjectList
type swaggerResponseProjectList struct {
	// in:body
	Body []api.Project `json:"body"`
}

// Column
// swagger:response Column
type swaggerResponseColumn struct {
	// in:body
	Body api.Column `json:"body"`
}

// ColumnList
// swagger:response ColumnList
type swaggerResponseColumnList struct {
	// in:body
	Body []api.Column `json:"body"`
}
