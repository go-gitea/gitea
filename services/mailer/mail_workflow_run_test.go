// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	actions_model "gitea.dev/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowRunJobStatusPresentation(t *testing.T) {
	// only done statuses reach the mail, see actions_model.Status.IsDone
	cases := []struct {
		status actions_model.Status
		icon   string
		class  string
	}{
		{actions_model.StatusSuccess, "✔", "status-success"},
		{actions_model.StatusFailure, "×", "status-failure"},
		{actions_model.StatusCancelled, "⊘", ""},
		{actions_model.StatusSkipped, "–", ""},
		{actions_model.StatusUnknown, "×", "status-failure"},
	}
	for _, c := range cases {
		t.Run(c.status.String(), func(t *testing.T) {
			icon, class := workflowRunJobStatusPresentation(c.status)
			assert.Equal(t, c.icon, icon)
			assert.Equal(t, c.class, class)
		})
	}
}
