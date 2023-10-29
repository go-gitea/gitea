// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

type (
	AutomationTriggerType      uint8
	AutomationActionType       uint8
	AutomationActionTargetType uint8
)

const (
	AutomationTriggerTypeMove              AutomationTriggerType = iota // 0
	AutomationTriggerTypeStatus                                         // 1
	AutomationTriggerTypeAssign                                         // 2
	AutomationTriggerTypeUnassign                                       // 3
	AutomationTriggerTypeLabel                                          // 4
	AutomationTriggerTypeUnlabel                                        // 5
	AutomationTriggerTypeAssignProject                                  // 6
	AutomationTriggerTypeAssignReviewer                                 // 7
	AutomationTriggerTypeUnassignReviewers                              // 8
	AutomationTriggerTypeApprove                                        // 9
	AutomationTriggerTypeXRef                                           // 10
)

const (
	AutomationActionTypeNoOperation       AutomationActionType = iota // 0
	AutomationActionTypeMove                                          // 1
	AutomationActionTypeStatus                                        // 2
	AutomationActionTypeAssign                                        // 3
	AutomationActionTypeUnassign                                      // 4
	AutomationActionTypeLabel                                         // 5
	AutomationActionTypeUnlabel                                       // 6
	AutomationActionTypeAssignProject                                 // 7
	AutomationActionTypeAssignReviewer                                // 8
	AutomationActionTypeUnassignReviewers                             // 9
	AutomationActionTypeApprove                                       // 10
)

const (
	AutomationActionTargetTypeDefault AutomationActionTargetType = (1 << iota) >> 1
	AutomationActionTargetTypeIssue
	AutomationActionTargetTypePullRequest
)

var automationTriggerTypeStringToID = map[string]AutomationTriggerType{
	"assign":             AutomationTriggerTypeAssign,
	"assign_project":     AutomationTriggerTypeAssignProject,
	"approve":            AutomationTriggerTypeApprove,
	"move":               AutomationTriggerTypeMove,
	"status":             AutomationTriggerTypeStatus,
	"label":              AutomationTriggerTypeLabel,
	"unlabel":            AutomationTriggerTypeUnlabel,
	"xref":               AutomationTriggerTypeXRef,
	"assign_reviewer":    AutomationTriggerTypeAssignReviewer,
	"unassign_reviewers": AutomationTriggerTypeUnassignReviewers,
	"unassign":           AutomationTriggerTypeUnassign,
}

var automationActionTypeStringToID = map[string]AutomationActionType{
	"label":              AutomationActionTypeLabel,
	"unlabel":            AutomationActionTypeUnlabel,
	"approve":            AutomationActionTypeApprove,
	"assign":             AutomationActionTypeAssign,
	"move":               AutomationActionTypeMove,
	"assign_project":     AutomationActionTypeAssignProject,
	"assign_reviewer":    AutomationActionTypeAssignReviewer,
	"status":             AutomationActionTypeStatus,
	"noop":               AutomationActionTypeNoOperation,
	"unassign":           AutomationActionTypeUnassign,
	"unassign_reviewers": AutomationActionTypeUnassignReviewers,
}

// TODO: add docstrings everywhere
type Automation struct {
	ID             int64                      `xorm:"pk autoincr"`
	Enabled        bool                       `xorm:"INDEX NOT NULL DEFAULT true"`
	ProjectID      int64                      `xorm:"INDEX NOT NULL"`
	ProjectBoardID int64                      `xorm:"INDEX NOT NULL"`
	TriggerType    AutomationTriggerType      `xorm:"INDEX NOT NULL"`
	TriggerData    int64                      `xorm:"INDEX NOT NULL DEFAULT 0"`
	ActionType     AutomationActionType       `xorm:"NOT NULL DEFAULT 0"`
	ActionData     int64                      `xorm:"NOT NULL DEFAULT 0"`
	ActionTarget   AutomationActionTargetType `xorm:"NOT NULL DEFAULT 0"`
	Sorting        int64                      `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type AutomationTrigger struct {
	Type AutomationTriggerType
	Data int64
}

type AutomationAction struct {
	Automation *Automation
	Type       AutomationActionType
	Data       int64
}

func (Automation) TableName() string {
	return "project_automation"
}

func init() {
	db.RegisterModel(new(Automation))
}

func (a *Automation) ShouldRunForTarget(target AutomationActionTargetType) bool {
	actionTarget := a.ActionTarget
	if a.ActionTarget == AutomationActionTargetTypeDefault {
		switch a.ActionType {
		case AutomationActionTypeAssignReviewer:
			fallthrough
		case AutomationActionTypeUnassignReviewers:
			fallthrough
		case AutomationActionTypeApprove:
			actionTarget = AutomationActionTargetTypePullRequest

		default:
			if a.TriggerType == AutomationTriggerTypeXRef {
				actionTarget = AutomationActionTargetTypeIssue
			} else {
				actionTarget = AutomationActionTargetTypeIssue | AutomationActionTargetTypePullRequest
			}
		}
	}
	return actionTarget&target != 0
}

func NewAutomation(ctx context.Context, automation *Automation) error {
	_, err := db.GetEngine(ctx).Insert(automation)
	return err
}

func FindAutomationsForTrigger(ctx context.Context, issueID int64, triggerType AutomationTriggerType, triggerID int64) ([]AutomationAction, error) {
	// Get the projects and the current issue state for those projects
	projectState := make([]*ProjectIssue, 0, 10)
	err := db.GetEngine(ctx).Table(ProjectIssue{}).
		Where("issue_id = ?", issueID).
		Find(&projectState)
	if err != nil {
		return nil, err
	}

	// Nothing to do if there are no projects
	if len(projectState) == 0 {
		return nil, nil
	}

	projectIDs := make([]int64, 0, len(projectState))
	for _, project := range projectState {
		projectIDs = append(projectIDs, project.ProjectID)
	}

	// Get all automations relating to issue
	automationTable := make([]*Automation, 0, 10)
	err = db.GetEngine(ctx).Table(Automation{}).
		In("project_id", projectIDs).
		Where("enabled = ?", true).
		Find(&automationTable)
	if err != nil {
		return nil, err
	}

	trigger := &AutomationTrigger{triggerType, triggerID}

	// Get the relevant actions for this trigger
	actions := getActions(automationTable, projectState, trigger)

	return actions, nil
}

func getProject(automation *Automation, projectState []*ProjectIssue) *ProjectIssue {
	for _, state := range projectState {
		if state.ProjectID == automation.ProjectID {
			return state
		}
	}
	return nil
}

func matchTrigger(automation *Automation, project *ProjectIssue, trigger *AutomationTrigger) bool {
	if automation.ProjectID != project.ProjectID ||
		automation.TriggerType != trigger.Type ||
		automation.TriggerData != trigger.Data {
		return false
	}
	if automation.ProjectBoardID != 0 {
		if automation.ProjectBoardID != project.ProjectBoardID {
			return false
		}
	}
	return true
}

func getActions(automationTable []*Automation, projectState []*ProjectIssue, trigger *AutomationTrigger) []AutomationAction {
	actions := make([]AutomationAction, 0, 10)
	for _, automation := range automationTable {
		project := getProject(automation, projectState)
		if project == nil || !matchTrigger(automation, project, trigger) {
			continue
		}
		actions = append(actions, AutomationAction{
			automation,
			automation.ActionType,
			automation.ActionData,
		})
	}
	return actions
}

func (p *Project) AutomationFromConfig(
	config map[string]string,
	lookupBoardID func(title string) (int64, error),
	lookupLabelID func(label string) (int64, error),
) (*Automation, error) {
	parseActionTarget := func(s string) AutomationActionTargetType {
		target := AutomationActionTargetTypeDefault
		for _, t := range strings.Split(s, ",") {
			switch t {
			case "issue":
				target |= AutomationActionTargetTypeIssue
			case "pr":
				target |= AutomationActionTargetTypePullRequest
			}
		}
		return target
	}

	parseArg := func(arg string, lookup func(string) (int64, error)) (int64, error) {
		data, _ := strconv.ParseInt(arg, 10, 64)
		if len(arg) > 0 && lookup != nil {
			data, err := lookup(arg)
			if err != nil {
				return 0, err
			}
			return data, nil
		}
		return data, nil
	}

	lookupStatus := func(s string) (int64, error) {
		switch s {
		case "closed":
			return 1, nil
		case "reopened":
			return 0, nil
		default:
			return 0, fmt.Errorf("unknown status %s", s)
		}
	}

	parseTrigger := func(s string) (AutomationTriggerType, int64, error) {
		cmd, arg, _ := strings.Cut(s, ":")
		if trigger, ok := automationTriggerTypeStringToID[cmd]; ok {
			data, _ := strconv.ParseInt(arg, 10, 64)
			var err error
			switch trigger {
			case AutomationTriggerTypeStatus:
				data, err = parseArg(arg, lookupStatus)
			case AutomationTriggerTypeMove:
				data, err = parseArg(arg, lookupBoardID)
			case AutomationTriggerTypeLabel:
				data, err = parseArg(arg, lookupLabelID)
			case AutomationTriggerTypeUnlabel:
				data, err = parseArg(arg, lookupLabelID)
			case AutomationTriggerTypeXRef:
				data = int64(references.XRefActionFromString(arg))
			}
			return trigger, data, err
		}
		return 0, 0, fmt.Errorf("could not parse trigger: %s", s)
	}

	parseAction := func(s string) (AutomationActionType, int64, error) {
		cmd, arg, _ := strings.Cut(s, ":")
		if action, ok := automationActionTypeStringToID[cmd]; ok {
			data, _ := strconv.ParseInt(arg, 10, 64)
			var err error
			switch action {
			case AutomationActionTypeStatus:
				data, err = parseArg(arg, lookupStatus)
			case AutomationActionTypeMove:
				data, err = parseArg(arg, lookupBoardID)
			case AutomationActionTypeLabel:
				data, err = parseArg(arg, lookupLabelID)
			case AutomationActionTypeUnlabel:
				data, err = parseArg(arg, lookupLabelID)
			}
			return action, data, err
		}
		return 0, 0, fmt.Errorf("could not parse action: %s", s)
	}

	var err error
	automation := &Automation{}
	automation.ProjectID = p.ID
	if automation.ProjectBoardID, err = parseArg(config["context"], lookupBoardID); err != nil {
		return nil, err
	}
	if automation.TriggerType, automation.TriggerData, err = parseTrigger(config["trigger"]); err != nil {
		return nil, err
	}
	if automation.ActionType, automation.ActionData, err = parseAction(config["action"]); err != nil {
		return nil, err
	}
	automation.ActionTarget = parseActionTarget(config["target"])

	return automation, nil
}

func (p *Project) AutomationToConfig(automation *Automation) (map[string]string, error) {
	return nil, nil
}

func createAutomationForProjectsType(ctx context.Context, project *Project, lookupLabelID func(label string) (int64, error)) error {
	var items []map[string]string

	switch project.BoardType {

	case BoardTypeAutomatedKanban:
		items = setting.ProjectAutomationKanban.GetConfig()

	case BoardTypeNone:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	boards, err := project.GetBoards(ctx)
	if err != nil {
		return err
	}

	automation := make([]*Automation, 0, len(items))
	lookupBoardID := func(s string) (int64, error) {
		for _, board := range boards {
			if board.Title == s {
				return board.ID, nil
			}
		}
		return 0, fmt.Errorf("board %s not found", s)
	}

	for _, v := range items {
		if a, err := project.AutomationFromConfig(v, lookupBoardID, lookupLabelID); a != nil && err == nil {
			a.Enabled = true
			a.CreatedUnix = timeutil.TimeStampNow()
			automation = append(automation, a)
		} else {
			log.Error("AutomationFromConfig: %v", err)
		}
	}

	if len(automation) == 0 {
		return nil
	}

	return db.Insert(ctx, automation)
}

func deleteAutomationByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&Automation{})
	return err
}

func deleteAutomationByBoardID(ctx context.Context, boardID int64) error {
	_, err := db.GetEngine(ctx).Where("project_board_id=?", boardID).Delete(&Automation{})
	return err
}
