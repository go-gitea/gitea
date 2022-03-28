// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
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
	Cancel      context.CancelFunc
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
