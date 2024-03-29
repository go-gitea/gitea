// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

type (
	// BoardViewType is used to represent a project column type
	BoardViewType uint8

	// BoardConfig is used to identify the type of board that is being created
	BoardConfig struct {
		BoardType   BoardViewType
		Translation string
	}
)

const (
	// BoardViewTypeNone is a project board type that has no predefined columns
	BoardViewTypeNone BoardViewType = iota

	// BoardViewTypeBasicKanban is a project board type that has basic predefined columns
	BoardViewTypeBasicKanban

	// BoardViewTypeBugTriage is a project board type that has predefined columns suited to hunting down bugs
	BoardViewTypeBugTriage
)

// GetBoardViewConfig retrieves the types of configurations project boards could have
func GetBoardViewConfig() []BoardConfig {
	return []BoardConfig{
		{BoardViewTypeNone, "repo.projects.type.none"},
		{BoardViewTypeBasicKanban, "repo.projects.type.basic_kanban"},
		{BoardViewTypeBugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsBoardViewTypeValid checks if the project board type is valid
func IsBoardViewTypeValid(p BoardViewType) bool {
	switch p {
	case BoardViewTypeNone, BoardViewTypeBasicKanban, BoardViewTypeBugTriage:
		return true
	default:
		return false
	}
}
