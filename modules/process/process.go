// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"time"
)

var (
	SystemProcessType  = "system"
	RequestProcessType = "request"
	NormalProcessType  = "normal"
	NoneProcessType    = "none"
)

// process represents a working process inheriting from Gitea.
type process struct {
	PID         IDType // Process ID, not system one.
	ParentPID   IDType
	Description string
	Start       time.Time
	Cancel      CancelCauseFunc
	Type        string
}

// ToProcess converts a process to a externally usable Process
func (p *process) toProcess() *Process {
	process := &Process{
		PID:         p.PID,
		ParentPID:   p.ParentPID,
		Description: p.Description,
		Start:       p.Start,
		Type:        p.Type,
	}
	return process
}
