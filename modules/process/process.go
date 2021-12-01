// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
	"sync"
	"time"
)

// Process represents a working process inheriting from Gitea.
type Process struct {
	PID         IDType // Process ID, not system one.
	ParentPID   IDType
	Description string
	Start       time.Time
	Cancel      context.CancelFunc

	lock     sync.Mutex
	children []*Process
}

// Children gets the children of the process
// Note: this function will behave nicely even if p is nil
func (p *Process) Children() (children []*Process) {
	if p == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	children = make([]*Process, len(p.children))
	copy(children, p.children)
	return children
}

// AddChild adds a child process
// Note: this function will behave nicely even if p is nil
func (p *Process) AddChild(child *Process) {
	if p == nil {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	p.children = append(p.children, child)
}

// RemoveChild removes a child process
// Note: this function will behave nicely even if p is nil
func (p *Process) RemoveChild(process *Process) {
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
