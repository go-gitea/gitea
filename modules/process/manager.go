// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

// TODO: This packages still uses a singleton for the Manager.
// Once there's a decent web framework and dependencies are passed around like they should,
// then we delete the singleton.

var (
	manager     *Manager
	managerInit sync.Once

	// DefaultContext is the default context to run processing commands in
	DefaultContext = context.Background()
)

// DescriptionPProfLabel is a label set on goroutines that have a process attached
const DescriptionPProfLabel = "process-description"

// PIDPProfLabel is a label set on goroutines that have a process attached
const PIDPProfLabel = "pid"

// PPIDPProfLabel is a label set on goroutines that have a process attached
const PPIDPProfLabel = "ppid"

// ProcessTypePProfLabel is a label set on goroutines that have a process attached
const ProcessTypePProfLabel = "process-type"

// IDType is a pid type
type IDType string

// FinishedFunc is a function that marks that the process is finished and can be removed from the process table
// - it is simply an alias for context.CancelFunc and is only for documentary purposes
type FinishedFunc = context.CancelFunc

// Manager manages all processes and counts PIDs.
type Manager struct {
	mutex sync.Mutex

	next     int64
	lastTime int64

	processMap map[IDType]*process
}

// GetManager returns a Manager and initializes one as singleton if there's none yet
func GetManager() *Manager {
	managerInit.Do(func() {
		manager = &Manager{
			processMap: make(map[IDType]*process),
			next:       1,
		}
	})
	return manager
}

// AddContext creates a new context and adds it as a process. Once the process is finished, finished must be called
// to remove the process from the process table. It should not be called until the process is finished but must always be called.
//
// cancel should be used to cancel the returned context, however it will not remove the process from the process table.
// finished will cancel the returned context and remove it from the process table.
//
// Most processes will not need to use the cancel function but there will be cases whereby you want to cancel the process but not immediately remove it from the
// process table.
func (pm *Manager) AddContext(parent context.Context, description string) (ctx context.Context, cancel context.CancelFunc, finished FinishedFunc) {
	ctx, cancel = context.WithCancel(parent)

	ctx, _, finished = pm.Add(ctx, description, cancel, NormalProcessType, true)

	return ctx, cancel, finished
}

// AddTypedContext creates a new context and adds it as a process. Once the process is finished, finished must be called
// to remove the process from the process table. It should not be called until the process is finished but must always be called.
//
// cancel should be used to cancel the returned context, however it will not remove the process from the process table.
// finished will cancel the returned context and remove it from the process table.
//
// Most processes will not need to use the cancel function but there will be cases whereby you want to cancel the process but not immediately remove it from the
// process table.
func (pm *Manager) AddTypedContext(parent context.Context, description, processType string, currentlyRunning bool) (ctx context.Context, cancel context.CancelFunc, finished FinishedFunc) {
	ctx, cancel = context.WithCancel(parent)

	ctx, _, finished = pm.Add(ctx, description, cancel, processType, currentlyRunning)

	return ctx, cancel, finished
}

// AddContextTimeout creates a new context and add it as a process. Once the process is finished, finished must be called
// to remove the process from the process table. It should not be called until the process is finished but must always be called.
//
// cancel should be used to cancel the returned context, however it will not remove the process from the process table.
// finished will cancel the returned context and remove it from the process table.
//
// Most processes will not need to use the cancel function but there will be cases whereby you want to cancel the process but not immediately remove it from the
// process table.
func (pm *Manager) AddContextTimeout(parent context.Context, timeout time.Duration, description string) (ctx context.Context, cancel context.CancelFunc, finshed FinishedFunc) {
	if timeout <= 0 {
		// it's meaningless to use timeout <= 0, and it must be a bug! so we must panic here to tell developers to make the timeout correct
		panic("the timeout must be greater than zero, otherwise the context will be cancelled immediately")
	}

	ctx, cancel = context.WithTimeout(parent, timeout)

	ctx, _, finshed = pm.Add(ctx, description, cancel, NormalProcessType, true)

	return ctx, cancel, finshed
}

// Add create a new process
func (pm *Manager) Add(ctx context.Context, description string, cancel context.CancelFunc, processType string, currentlyRunning bool) (context.Context, IDType, FinishedFunc) {
	parentPID := GetParentPID(ctx)

	pm.mutex.Lock()
	start, pid := pm.nextPID()

	parent := pm.processMap[parentPID]
	if parent == nil {
		parentPID = ""
	}

	process := &process{
		PID:         pid,
		ParentPID:   parentPID,
		Description: description,
		Start:       start,
		Cancel:      cancel,
		Type:        processType,
	}

	var finished FinishedFunc
	if currentlyRunning {
		finished = func() {
			cancel()
			pm.remove(process)
			pprof.SetGoroutineLabels(ctx)
		}
	} else {
		finished = func() {
			cancel()
			pm.remove(process)
		}
	}

	pm.processMap[pid] = process
	pm.mutex.Unlock()

	pprofCtx := pprof.WithLabels(ctx, pprof.Labels(DescriptionPProfLabel, description, PPIDPProfLabel, string(parentPID), PIDPProfLabel, string(pid), ProcessTypePProfLabel, processType))
	if currentlyRunning {
		pprof.SetGoroutineLabels(pprofCtx)
	}

	return &Context{
		Context: pprofCtx,
		pid:     pid,
	}, pid, finished
}

// nextPID will return the next available PID. pm.mutex should already be locked.
func (pm *Manager) nextPID() (start time.Time, pid IDType) {
	start = time.Now()
	startUnix := start.Unix()
	if pm.lastTime == startUnix {
		pm.next++
	} else {
		pm.next = 1
	}
	pm.lastTime = startUnix
	pid = IDType(strconv.FormatInt(start.Unix(), 16))

	if pm.next == 1 {
		return
	}
	pid = IDType(string(pid) + "-" + strconv.FormatInt(pm.next, 10))
	return
}

// Remove a process from the ProcessManager.
func (pm *Manager) Remove(pid IDType) {
	pm.mutex.Lock()
	delete(pm.processMap, pid)
	pm.mutex.Unlock()
}

func (pm *Manager) remove(process *process) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	if p := pm.processMap[process.PID]; p == process {
		delete(pm.processMap, process.PID)
	}
}

// Cancel a process in the ProcessManager.
func (pm *Manager) Cancel(pid IDType) {
	pm.mutex.Lock()
	process, ok := pm.processMap[pid]
	pm.mutex.Unlock()
	if ok && process.Type != SystemProcessType {
		process.Cancel()
	}
}
