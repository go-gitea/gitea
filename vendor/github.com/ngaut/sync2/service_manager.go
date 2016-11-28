// Copyright 2013, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync2

import (
	"sync"
)

// These are the three predefined states of a service.
const (
	SERVICE_STOPPED = iota
	SERVICE_RUNNING
	SERVICE_SHUTTING_DOWN
)

var stateNames = []string{
	"Stopped",
	"Running",
	"ShuttingDown",
}

// ServiceManager manages the state of a service through its lifecycle.
type ServiceManager struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	err   error // err is the error returned from the service function.
	state AtomicInt64
	// shutdown is created when the service starts and is closed when the service
	// enters the SERVICE_SHUTTING_DOWN state.
	shutdown chan struct{}
}

// Go tries to change the state from SERVICE_STOPPED to SERVICE_RUNNING.
//
// If the current state is not SERVICE_STOPPED (already running), it returns
// false immediately.
//
// On successful transition, it launches the service as a goroutine and returns
// true. The service function is responsible for returning on its own when
// requested, either by regularly checking svc.IsRunning(), or by waiting for
// the svc.ShuttingDown channel to be closed.
//
// When the service func returns, the state is reverted to SERVICE_STOPPED.
func (svm *ServiceManager) Go(service func(svc *ServiceContext) error) bool {
	svm.mu.Lock()
	defer svm.mu.Unlock()
	if !svm.state.CompareAndSwap(SERVICE_STOPPED, SERVICE_RUNNING) {
		return false
	}
	svm.wg.Add(1)
	svm.err = nil
	svm.shutdown = make(chan struct{})
	go func() {
		svm.err = service(&ServiceContext{ShuttingDown: svm.shutdown})
		svm.state.Set(SERVICE_STOPPED)
		svm.wg.Done()
	}()
	return true
}

// Stop tries to change the state from SERVICE_RUNNING to SERVICE_SHUTTING_DOWN.
// If the current state is not SERVICE_RUNNING, it returns false immediately.
// On successul transition, it waits for the service to finish, and returns true.
// You are allowed to Go() again after a Stop().
func (svm *ServiceManager) Stop() bool {
	svm.mu.Lock()
	defer svm.mu.Unlock()
	if !svm.state.CompareAndSwap(SERVICE_RUNNING, SERVICE_SHUTTING_DOWN) {
		return false
	}
	// Signal the service that we've transitioned to SERVICE_SHUTTING_DOWN.
	close(svm.shutdown)
	svm.shutdown = nil
	svm.wg.Wait()
	return true
}

// Wait waits for the service to terminate if it's currently running.
func (svm *ServiceManager) Wait() {
	svm.wg.Wait()
}

// Join waits for the service to terminate and returns the value returned by the
// service function.
func (svm *ServiceManager) Join() error {
	svm.wg.Wait()
	return svm.err
}

// State returns the current state of the service.
// This should only be used to report the current state.
func (svm *ServiceManager) State() int64 {
	return svm.state.Get()
}

// StateName returns the name of the current state.
func (svm *ServiceManager) StateName() string {
	return stateNames[svm.State()]
}

// ServiceContext is passed into the service function to give it access to
// information about the running service.
type ServiceContext struct {
	// ShuttingDown is a channel that the service can select on to be notified
	// when it should shut down. The channel is closed when the state transitions
	// from SERVICE_RUNNING to SERVICE_SHUTTING_DOWN.
	ShuttingDown chan struct{}
}

// IsRunning returns true if the ServiceContext.ShuttingDown channel has not
// been closed yet.
func (svc *ServiceContext) IsRunning() bool {
	select {
	case <-svc.ShuttingDown:
		return false
	default:
		return true
	}
}
