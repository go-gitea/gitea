// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
)

type ProjectAutomationConfig struct {
	Rule1  []string
	Rule2  []string
	Rule3  []string
	Rule4  []string
	Rule5  []string
	Rule6  []string
	Rule7  []string
	Rule8  []string
	Rule9  []string
	Rule10 []string
	Rule11 []string
	Rule12 []string
	Rule13 []string
	Rule14 []string
	Rule15 []string
	Rule16 []string
	Rule17 []string
	Rule18 []string
	Rule19 []string
	Rule20 []string
	Rule21 []string
	Rule22 []string
	Rule23 []string
	Rule24 []string
	Rule25 []string
}

// Project settings
var (
	Project = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
	}{
		ProjectBoardBasicKanbanType: []string{"To Do", "In Progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs Triage", "High Priority", "Low Priority", "Closed"},
	}
	ProjectAutomation = struct {
		Enabled            bool
		MaxRulesPerProject int
	}{
		Enabled:            false,
		MaxRulesPerProject: 25,
	}
	ProjectAutomationKanban = ProjectAutomationConfig{
		Rule1:  []string{"trigger=status:closed", "action=move:Done"},
		Rule2:  []string{"context=Done", "trigger=status:reopened", "action=move:To Do"},
		Rule3:  []string{"trigger=approve", "target=pr", "action=move:Done"},
		Rule4:  []string{"trigger=xref:closes", "target=pr", "action=assign_project"},
		Rule5:  []string{"trigger=assign_project", "target=pr", "action=move:To Do"},
		Rule6:  []string{"trigger=move:To Do", "action=unassign"},
		Rule7:  []string{"trigger=move:To Do", "action=status:reopened"},
		Rule8:  []string{"trigger=move:To Do", "action=unassign_reviewers"},
		Rule9:  []string{"trigger=move:In Progress", "action=assign"},
		Rule10: []string{"trigger=move:In Progress", "action=status:reopened"},
		Rule11: []string{"trigger=move:In Progress", "action=assign_reviewer"},
		Rule12: []string{"trigger=move:Done", "action=status:closed", "target=issue"},
		Rule13: []string{"trigger=move:Done", "action=approve", "target=pr"},
		Rule14: []string{"context=To Do", "trigger=assign", "target=issue", "action=move:In Progress"},
		Rule15: []string{"context=To Do", "trigger=assign_reviewer", "action=move:In Progress"},
		Rule16: []string{"context=In Progress", "trigger=xref:closes", "target=issue", "action=label:Status/Blocked"},
		Rule17: []string{"context=In Progress", "trigger=approve", "target=issue", "action=unlabel:Status/Blocked"},
	}
)

func loadProjectFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "project", &Project)
	mustMapSetting(rootCfg, "project.automation", &ProjectAutomation)
	mustMapSetting(rootCfg, "project.automation.kanban", &ProjectAutomationKanban)
}

func (conf *ProjectAutomationConfig) GetConfig() []map[string]string {
	config := []map[string]string{}
	for _, rules := range [][]string{
		conf.Rule1, conf.Rule2, conf.Rule3, conf.Rule4, conf.Rule5,
		conf.Rule6, conf.Rule7, conf.Rule8, conf.Rule9, conf.Rule10,
		conf.Rule11, conf.Rule12, conf.Rule13, conf.Rule14, conf.Rule15,
		conf.Rule16, conf.Rule17, conf.Rule18, conf.Rule19, conf.Rule20,
		conf.Rule21, conf.Rule22, conf.Rule23, conf.Rule24, conf.Rule25,
	} {
		ruleConfig := map[string]string{}
		for _, rule := range rules {
			if key, value, ok := strings.Cut(rule, "="); ok {
				ruleConfig[key] = value
			}
		}
		if len(ruleConfig) > 0 {
			config = append(config, ruleConfig)
		}
	}
	return config
}
