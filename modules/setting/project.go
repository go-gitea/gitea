// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Project settings
var (
	Project = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
		ProjectBoardDefault         string
	}{
		ProjectBoardBasicKanbanType: []string{"To Do", "In Progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs Triage", "High Priority", "Low Priority", "Closed"},
		ProjectBoardDefault:         "Uncategorized",
	}
)

func loadProjectFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "project", &Project)
}
