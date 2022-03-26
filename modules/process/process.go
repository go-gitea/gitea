// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
	"sync"
	"time"
)

var (
	SystemProcessType  = "system"
	RequestProcessType = "request"
	NormalProcessType  = "normal"
)

// process represents a working process inheriting from Gitea.
type process struct {
	PID         IDType // Process ID, not system one.
	ParentPID   IDType
	Description string
	Start       time.Time
	Cancel      context.CancelFunc
	Type        string

	lock     sync.Mutex
	children []*process
}

// Children gets the children of the process
// Note: this function will behave nicely even if p is nil
func (p *process) Children() (children []*process) {
	if p == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	children = make([]*process, len(p.children))
	copy(children, p.children)
	return children
}

// AddChild adds a child process
// Note: this function will behave nicely even if p is nil
func (p *process) AddChild(child *process) {
	if p == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	p.children = append(p.children, child)
}

// RemoveChild removes a child process
// Note: this function will behave nicely even if p is nil
func (p *process) RemoveChild(process *process) {
	if p == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	for i, child := range p.children {
		if child == process {
			p.children = append(p.children[:i], p.children[i+1:]...)
			return
		}
	}
}

// ToProcess converts a process to a externally usable Process
func (p *process) ToProcess(children bool) *Process {
	process := &Process{
		PID:         p.PID,
		ParentPID:   p.ParentPID,
		Description: p.Description,
		Start:       p.Start,
		Type:        p.Type,
	}
	if children {
		for _, child := range p.Children() {
			process.Children = append(process.Children, child.ToProcess(children))
		}
	}
	return process
}
