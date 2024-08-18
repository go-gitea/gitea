// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sync

import (
	"sync"

	"code.gitea.io/gitea/modules/container"
)

// StatusTable is a table maintains true/false values.
//
// This table is particularly useful for un/marking and checking values
// in different goroutines.
type StatusTable struct {
	lock sync.RWMutex
	pool container.Set[string]
}

// NewStatusTable initializes and returns a new StatusTable object.
func NewStatusTable() *StatusTable {
	return &StatusTable{
		pool: make(container.Set[string]),
	}
}

// StartIfNotRunning sets value of given name to true if not already in pool.
// Returns whether set value was set to true
func (p *StatusTable) StartIfNotRunning(name string) bool {
	p.lock.Lock()
	added := p.pool.Add(name)
	p.lock.Unlock()
	return added
}

// Start sets value of given name to true in the pool.
func (p *StatusTable) Start(name string) {
	p.lock.Lock()
	p.pool.Add(name)
	p.lock.Unlock()
}

// Stop sets value of given name to false in the pool.
func (p *StatusTable) Stop(name string) {
	p.lock.Lock()
	p.pool.Remove(name)
	p.lock.Unlock()
}

// IsRunning checks if value of given name is set to true in the pool.
func (p *StatusTable) IsRunning(name string) bool {
	p.lock.RLock()
	exists := p.pool.Contains(name)
	p.lock.RUnlock()
	return exists
}
