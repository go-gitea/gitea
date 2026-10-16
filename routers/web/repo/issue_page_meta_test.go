// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestSetSelectedAssigneesFromQuery(t *testing.T) {
	newMetaData := func() *IssuePageMetaData {
		return &IssuePageMetaData{AssigneesData: &issueSidebarAssigneesData{
			SelectedAssigneeIDs: "2",
			CandidateAssignees:  []*user_model.User{{ID: 2}, {ID: 4}, {ID: 5}},
		}}
	}

	ctx, _ := contexttest.MockContext(t, "user2/repo1/compare/master...branch2?assignee=5,3,4")
	d := newMetaData()
	d.setSelectedAssigneesFromQuery(ctx)
	assert.Equal(t, "5,4", d.AssigneesData.SelectedAssigneeIDs)

	ctx, _ = contexttest.MockContext(t, "user2/repo1/compare/master...branch2")
	d = newMetaData()
	d.setSelectedAssigneesFromQuery(ctx)
	assert.Equal(t, "2", d.AssigneesData.SelectedAssigneeIDs)
}
