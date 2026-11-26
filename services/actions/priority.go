// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"
)

// parseJobPriority treats an invalid value as normal, workflows are validated before they run.
func parseJobPriority(jobID string, workflow *jobparser.SingleWorkflow) int {
	level, err := jobparser.ParsePriority(workflow.Priority)
	if err != nil {
		log.Warn("job %q: %v, using normal", jobID, err)
	}
	return level
}
