// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package releasereopen

import (
	"errors"
	"sync"
)

type ReleaseReopener interface {
	ReleaseReopen() error
}

type Manager struct {
	mu      sync.Mutex
	counter int64

	releaseReopeners map[int64]ReleaseReopener
}

func (r *Manager) Register(rr ReleaseReopener) (cancel func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	currentCounter := r.counter
	r.releaseReopeners[r.counter] = rr

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		delete(r.releaseReopeners, currentCounter)
	}
}

func (r *Manager) ReleaseReopen() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, rr := range r.releaseReopeners {
		if err := rr.ReleaseReopen(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func GetManager() *Manager {
	return manager
}

func NewManager() *Manager {
	return &Manager{
		releaseReopeners: make(map[int64]ReleaseReopener),
	}
}

var manager = NewManager()
