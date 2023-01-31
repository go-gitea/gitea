// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/log"

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

func newProject() {
	if err := Cfg.Section("project").MapTo(&Project); err != nil {
		log.Fatal("Failed to map Project settings: %v", err)
	}
}
