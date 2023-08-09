// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Project settings
var (
	Project = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
	}{
		ProjectBoardBasicKanbanType: []string{"To Do", "In Progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs Triage", "High Priority", "Low Priority", "Closed"},
	}
)

func loadProjectFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "project", &Project)
}
