// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/log"

// Board settings
var (
	Board = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
	}{
		ProjectBoardBasicKanbanType: []string{"To Do", "In Progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs Triage", "High Priority", "Low Priority", "Closed"},
	}
)

func newBoard() {
	if err := Cfg.Section("project").MapTo(&Board); err != nil {
		log.Fatal("Failed to map Board settings: %v", err)
	}
}
