// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
)

// Project settings
var (
	Project = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
	}{
		ProjectBoardBasicKanbanType: []string{"To Do", "In Progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs Triage", "High priority", "Low priority", "Closed"},
	}
)

func newProject() {
	sec := Cfg.Section("project")
	Project.ProjectBoardBasicKanbanType = strings.Split(sec.Key("PROJECT_BOARD_BASIC_KANBAN_TYPE").MustString("To Do,In Progress,Done"), ",")
	Project.ProjectBoardBugTriageType = strings.Split(sec.Key("PROJECT_BOARD_BUG_TRIAGE_TYPE").MustString("Needs Triage,High priority,Low priority,Closed"), ",")
}
