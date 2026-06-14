// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
)

func TestGetRunWorkflowIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := GetRunWorkflowIDs(t.Context(), 4)
	assert.NoError(t, err)
	assert.Equal(t, []string{"artifact.yaml", "test.yaml"}, ids)

	ids, err = GetRunWorkflowIDs(t.Context(), 999999)
	assert.NoError(t, err)
	assert.Empty(t, ids)
}

func TestGetStatusInfoList(t *testing.T) {
	statusInfoList := GetStatusInfoList(t.Context(), translation.MockLocale{})

	assert.Equal(t, []StatusInfo{
		{Status: int(StatusSuccess), StatusName: StatusSuccess.String(), DisplayedStatus: "actions.status.success"},
		{Status: int(StatusFailure), StatusName: StatusFailure.String(), DisplayedStatus: "actions.status.failure"},
		{Status: int(StatusWaiting), StatusName: StatusWaiting.String(), DisplayedStatus: "actions.status.waiting"},
		{Status: int(StatusRunning), StatusName: StatusRunning.String(), DisplayedStatus: "actions.status.running"},
		{Status: int(StatusCancelling), StatusName: StatusCancelling.String(), DisplayedStatus: "actions.status.cancelling"},
	}, statusInfoList)
}
