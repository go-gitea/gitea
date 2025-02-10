// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "code.gitea.io/gitea/services/context"

// WorkflowAPI for action workflow of a repository
type WorkflowAPI interface {
	// ListRepositoryWorkflows list repository workflows
	ListRepositoryWorkflows(*context.APIContext)
	// GetWorkflow get a workflow
	GetWorkflow(*context.APIContext)
	// DisableWorkflow disable a workflow
	DisableWorkflow(*context.APIContext)
	// DispatchWorkflow create a workflow dispatch event
	DispatchWorkflow(*context.APIContext)
	// EnableWorkflow enable a workflow
	EnableWorkflow(*context.APIContext)
}
