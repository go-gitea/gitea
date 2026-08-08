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
		alt    string
		class  string
	}{
		{actions_model.StatusSuccess, "status-success.png", "✔", "status-success"},
		{actions_model.StatusFailure, "status-failure.png", "×", "status-failure"},
		{actions_model.StatusCancelled, "status-cancelled.png", "⊘", ""},
		{actions_model.StatusSkipped, "status-skipped.png", "–", ""},
		{actions_model.StatusUnknown, "status-failure.png", "×", "status-failure"},
	}
	for _, c := range cases {
		t.Run(c.status.String(), func(t *testing.T) {
			icon, alt, class := workflowRunJobStatusPresentation(c.status)
			assert.Equal(t, c.icon, icon)
			assert.Equal(t, c.alt, alt)
			assert.Equal(t, c.class, class)
			content, err := LoadMailIcon(icon)
			assert.NoError(t, err)
			assert.NotEmpty(t, content)
		})
	}
}
