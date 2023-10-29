// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/references"

	"github.com/stretchr/testify/assert"
)

func TestAutomationFromConfig(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project, _ := GetProjectByID(db.DefaultContext, 1)
	lookupBoardID := func(title string) (int64, error) {
		switch title {
		case "Done":
			return 3, nil
		case "To Do":
			return 5, nil
		case "In Progress":
			return 7, nil
		}
		return 0, fmt.Errorf("unknown board: %s", title)
	}
	lookupLabelID := func(label string) (int64, error) {
		switch label {
		case "Status/Blocked":
			return 10, nil
		case "Status/Invalid":
			return 12, nil
		}
		return 0, fmt.Errorf("unknown label: %s", label)
	}

	tests := []struct {
		config map[string]string
		result Automation
	}{
		{
			config: map[string]string{"trigger": "label:Status/Invalid", "action": "move:Done"},
			result: Automation{
				ProjectID:      1,
				ProjectBoardID: 0,
				TriggerType:    AutomationTriggerTypeLabel,
				TriggerData:    12,
				ActionType:     AutomationActionTypeMove,
				ActionData:     3,
				ActionTarget:   AutomationActionTargetTypeDefault,
			},
		},
		{
			config: map[string]string{"trigger": "move:Done", "action": "status:closed", "target": "issue,pr"},
			result: Automation{
				ProjectID:      1,
				ProjectBoardID: 0,
				TriggerType:    AutomationTriggerTypeMove,
				TriggerData:    3,
				ActionType:     AutomationActionTypeStatus,
				ActionData:     1,
				ActionTarget:   AutomationActionTargetTypeIssue | AutomationActionTargetTypePullRequest,
			},
		},
		{
			config: map[string]string{"context": "In Progress", "trigger": "xref:closes", "action": "label:Status/Blocked", "target": "issue"},
			result: Automation{
				ProjectID:      1,
				ProjectBoardID: 7,
				TriggerType:    AutomationTriggerTypeXRef,
				TriggerData:    int64(references.XRefActionCloses),
				ActionType:     AutomationActionTypeLabel,
				ActionData:     10,
				ActionTarget:   AutomationActionTargetTypeIssue,
			},
		},
	}

	for _, test := range tests {
		result, err := project.AutomationFromConfig(test.config, lookupBoardID, lookupLabelID)
		assert.NoError(t, err)
		assert.NotNil(t, result, "Expected result")
		assert.Equal(t, test.result.ProjectID, result.ProjectID, "ProjectID mismatch")
		assert.Equal(t, test.result.ProjectBoardID, result.ProjectBoardID, "ProjectBoardID mismatch")
		assert.Equal(t, test.result.TriggerType, result.TriggerType, "TriggerType mismatch")
		assert.Equal(t, test.result.TriggerData, result.TriggerData, "TriggerData mismatch")
		assert.Equal(t, test.result.ActionType, result.ActionType, "ActionType mismatch")
		assert.Equal(t, test.result.ActionData, result.ActionData, "ActionData mismatch")
		assert.Equal(t, test.result.ActionTarget, result.ActionTarget, "ActionTarget mismatch")
	}

	config := map[string]string{"trigger": "move:Nonexisting board", "action": "noop"}
	result, err := project.AutomationFromConfig(config, lookupBoardID, lookupLabelID)
	assert.EqualError(t, err, "unknown board: Nonexisting board")
	assert.Nil(t, result, "Expected nil result on error")

	config = map[string]string{"context": "Nonexisting board", "trigger": "closed", "action": "noop"}
	result, err = project.AutomationFromConfig(config, lookupBoardID, lookupLabelID)
	assert.EqualError(t, err, "unknown board: Nonexisting board")
	assert.Nil(t, result, "Expected nil result on error")

	config = map[string]string{"trigger": "label:Nonexisting label", "action": "noop"}
	result, err = project.AutomationFromConfig(config, lookupBoardID, lookupLabelID)
	assert.EqualError(t, err, "unknown label: Nonexisting label")
	assert.Nil(t, result, "Expected nil result on error")

	config = map[string]string{"trigger": "unknown_trigger", "action": "noop"}
	result, err = project.AutomationFromConfig(config, nil, nil)
	assert.EqualError(t, err, "could not parse trigger: unknown_trigger")
	assert.Nil(t, result, "Expected nil result on error")

	config = map[string]string{"trigger": "status:closed", "action": "unknown_action"}
	result, err = project.AutomationFromConfig(config, nil, nil)
	assert.EqualError(t, err, "could not parse action: unknown_action")
	assert.Nil(t, result, "Expected nil result on error")
}
