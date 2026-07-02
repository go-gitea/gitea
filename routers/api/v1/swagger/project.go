// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "gitea.dev/modules/structs"

// ProjectWorkflow
// swagger:response ProjectWorkflow
type swaggerResponseProjectWorkflow struct {
	// in:body
	Body api.ProjectWorkflow `json:"body"`
}

// ProjectWorkflowList
// swagger:response ProjectWorkflowList
type swaggerResponseProjectWorkflowList struct {
	// in:body
	Body []api.ProjectWorkflow `json:"body"`
}

// ProjectWorkflowOptions
// swagger:response ProjectWorkflowOptions
type swaggerResponseProjectWorkflowOptions struct {
	// in:body
	Body api.ProjectWorkflowOptions `json:"body"`
}
