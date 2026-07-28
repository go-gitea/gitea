// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strconv"
	"strings"

	"gitea.dev/modules/structs"
)

const (
	GhostUserID   int64 = -1
	GhostUserName       = "Ghost"
)

// NewGhostUser creates and returns a fake user for someone has deleted their account.
func NewGhostUser() *User {
	return &User{
		ID:        GhostUserID,
		Name:      GhostUserName,
		LowerName: strings.ToLower(GhostUserName),
	}
}

// IsGhost check if user is fake user for a deleted account
func (u *User) IsGhost() bool {
	if u == nil {
		return false
	}
	return u.ID == GhostUserID && u.Name == GhostUserName
}

const (
	// ProjectWorkflowUserID must NOT be GhostUserID (-1): comments created by a
	// project workflow action store this as PosterID, and GetPossibleUserFromMap/
	// GetPossibleUserByID special-case GhostUserID, so reusing it made every
	// workflow-generated comment indistinguishable from one left by a deleted user.
	ProjectWorkflowUserID   int64 = -3
	ProjectWorkflowDoerName       = "gitea-project-workflow"
)

// NewProjectWorkflowUser creates and returns the virtual actor used when a
// project workflow action (e.g. closing an issue, changing labels) is performed
// on behalf of a project rather than a real user.
func NewProjectWorkflowUser() *User {
	return &User{
		ID:         ProjectWorkflowUserID,
		Name:       ProjectWorkflowDoerName,
		LowerName:  ProjectWorkflowDoerName,
		FullName:   "Gitea Project Workflow",
		IsActive:   true,
		Type:       UserTypeBot,
		Visibility: structs.VisibleTypePublic,
	}
}

// IsProjectWorkflowUser check if user is the virtual actor for project workflow actions
func (u *User) IsProjectWorkflowUser() bool {
	return u != nil && u.ID == ProjectWorkflowUserID
}

const (
	ActionsUserID    int64 = -2
	ActionsUserName        = "gitea-actions"
	ActionsUserEmail       = "teabot@gitea.io"
)

// NewActionsUser creates and returns a fake user for running the actions.
func NewActionsUser() *User {
	return &User{
		ID:               ActionsUserID,
		Name:             ActionsUserName,
		LowerName:        ActionsUserName,
		IsActive:         true,
		FullName:         "Gitea Actions",
		Email:            ActionsUserEmail,
		KeepEmailPrivate: true,
		LoginName:        ActionsUserName,
		Type:             UserTypeBot,
		Visibility:       structs.VisibleTypePublic,
	}
}

func NewActionsUserWithTaskID(id int64) *User {
	u := NewActionsUser()
	// LoginName is for only internal usage in this case, so it can be moved to other fields in the future
	// TODO: refactor to u.ExtDoerData
	u.LoginSource = -1
	u.LoginName = "@" + ActionsUserName + "/" + strconv.FormatInt(id, 10)
	return u
}

func GetActionsUserTaskID(u *User) (int64, bool) {
	// TODO: refactor to u.ExtDoerData
	if u == nil || u.ID != ActionsUserID {
		return 0, false
	}
	prefix, payload, _ := strings.Cut(u.LoginName, "/")
	if prefix != "@"+ActionsUserName {
		return 0, false
	} else if taskID, err := strconv.ParseInt(payload, 10, 64); err == nil {
		return taskID, true
	}
	return 0, false
}

func (u *User) IsGiteaActions() bool {
	return u != nil && u.ID == ActionsUserID
}

func GetSystemUserByName(name string) *User {
	if strings.EqualFold(name, GhostUserName) {
		return NewGhostUser()
	}
	if strings.EqualFold(name, ActionsUserName) {
		return NewActionsUser()
	}
	if strings.EqualFold(name, ProjectWorkflowDoerName) {
		return NewProjectWorkflowUser()
	}
	return nil
}
