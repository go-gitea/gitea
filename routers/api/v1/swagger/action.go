// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "code.gitea.io/gitea/modules/structs"

// SecretList
// swagger:response SecretList
type swaggerResponseSecretList struct {
	// in:body
	Body []api.Secret `json:"body"`
}

// Secret
// swagger:response Secret
type swaggerResponseSecret struct {
	// in:body
	Body api.Secret `json:"body"`
}

// ActionVariable
// swagger:response ActionVariable
type swaggerResponseActionVariable struct {
	// in:body
	Body api.ActionVariable `json:"body"`
}

// VariableList
// swagger:response VariableList
type swaggerResponseVariableList struct {
	// in:body
	Body []api.ActionVariable `json:"body"`
}

// ActionWorkflow
// swagger:response ActionWorkflow
type swaggerResponseActionWorkflow struct {
	// in:body
	Body api.ActionWorkflow `json:"body"`
}

// ActionWorkflowList
// swagger:response ActionWorkflowList
type swaggerResponseActionWorkflowList struct {
	// in:body
	Body api.ActionWorkflowResponse `json:"body"`
}

// WorkflowRunRerunRequest
// swagger:model WorkflowRunRerunRequest
type swaggerWorkflowRunRerunRequest struct {
	// Enable debug logging for the re-run
	EnableDebugLogging bool `json:"enable_debug_logging"`
}

// WorkflowRunLogsRequest
// swagger:model WorkflowRunLogsRequest
type swaggerWorkflowRunLogsRequest struct {
	// Log cursors for incremental log streaming
	LogCursors []map[string]any `json:"logCursors"`
}
