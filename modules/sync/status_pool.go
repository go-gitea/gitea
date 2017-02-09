// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync"
)

// StatusTable is a table maintains true/false values.
//
// This table is particularly useful for un/marking and checking values
// in different goroutines.
type StatusTable struct {
	lock sync.RWMutex
	pool map[string]struct{}
}

// NewStatusTable initializes and returns a new StatusTable object.
func NewStatusTable() *StatusTable {
	return &StatusTable{
		pool: make(map[string]struct{}),
	}
}

// Start sets value of given name to true in the pool.
func (p *StatusTable) Start(name string) {
	p.lock.Lock()
	p.pool[name] = struct{}{}
	p.lock.Unlock()
}

// Stop sets value of given name to false in the pool.
func (p *StatusTable) Stop(name string) {
	p.lock.Lock()
	delete(p.pool, name)
	p.lock.Unlock()
}

// IsRunning checks if value of given name is set to true in the pool.
func (p *StatusTable) IsRunning(name string) bool {
	p.lock.RLock()
	_, ok := p.pool[name]
	p.lock.RUnlock()
	return ok
}
