// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
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

	// RecyclePID is the PID number at which we will attempt to recycle PIDs
	RecyclePID int64 = 1 << 16

	// HuntSize is the size of the hunt for the lowest free PID
	HuntSize int64 = 512
)

// Process represents a working process inheriting from Gitea.
type Process struct {
	PID         int64 // Process ID, not system one.
	ParentPID   int64
	Children    []*Process // FIXME: access may need to be locked!
	Description string
	Start       time.Time
	Cancel      context.CancelFunc
}

// Manager knows about all processes and counts PIDs.
type Manager struct {
	mutex sync.Mutex

	next int64
	low  int64

	processes map[int64]*Process
}

// GetManager returns a Manager and initializes one as singleton if there's none yet
func GetManager() *Manager {
	managerInit.Do(func() {
		manager = &Manager{
			processes: make(map[int64]*Process),
			next:      1,
			low:       1,
		}
	})
	return manager
}

// AddContext create a new context and add it as a process. The CancelFunc must always be called even if the context is Done()
func (pm *Manager) AddContext(parent context.Context, description string) (context.Context, context.CancelFunc) {
	parentPID := GetParentPID(parent)

	ctx, cancel := context.WithCancel(parent)

	pid, cancel := pm.Add(parentPID, description, cancel)

	return &Context{
		Context: ctx,
		pid:     pid,
	}, cancel
}

// AddContextTimeout create a new context and add it as a process
func (pm *Manager) AddContextTimeout(parent context.Context, timeout time.Duration, description string) (context.Context, context.CancelFunc) {
	parentPID := GetParentPID(parent)

	ctx, cancel := context.WithTimeout(parent, timeout)

	pid, cancel := pm.Add(parentPID, description, cancel)

	return &Context{
		Context: ctx,
		pid:     pid,
	}, cancel
}

// Add create a new process
func (pm *Manager) Add(parentPID int64, description string, cancel context.CancelFunc) (int64, context.CancelFunc) {
	pm.mutex.Lock()
	pid := pm.nextPID()

	parent := pm.processes[parentPID]
	if parent == nil {
		parentPID = 0
	}

	process := &Process{
		PID:         pid,
		ParentPID:   parentPID,
		Description: description,
		Start:       time.Now(),
	}

	process.Cancel = func() {
		cancel()
		pm.remove(process)
	}

	if parent != nil {
		parent.Children = append(parent.Children, process)
	}
	pm.processes[pid] = process
	pm.mutex.Unlock()

	return pid, process.Cancel
}

// nextPID will return the next available PID. pm.mutex should already be locked.
func (pm *Manager) nextPID() int64 {
	if pm.next > RecyclePID {
		for i := int64(0); i < HuntSize; i++ {
			if pm.low >= pm.next {
				pm.low = 1
				break
			}
			if _, ok := pm.processes[pm.low]; !ok {
				next := pm.low
				pm.low++
				return next
			}
			pm.low++
		}
	}
	next := pm.next
	pm.next++
	return next
}

// releasePID will release the PID. pm.mutex should already be locked.
func (pm *Manager) releasePID(pid int64) {
	if pid < pm.low+RecyclePID {
		pm.low = pid
	}
}

// Remove a process from the ProcessManager.
func (pm *Manager) Remove(pid int64) {
	pm.mutex.Lock()
	delete(pm.processes, pid)
	pm.releasePID(pid)
	pm.mutex.Unlock()
}

func (pm *Manager) remove(process *Process) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	if p := pm.processes[process.PID]; p == process {
		delete(pm.processes, process.PID)
		pm.releasePID(process.PID)
		for _, child := range process.Children {
			child.ParentPID = 0
		}
		parent := pm.processes[process.ParentPID]
		if parent != nil {
			for i, child := range parent.Children {
				if child == process {
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					return
				}
			}
		}
	}
}

// Cancel a process in the ProcessManager.
func (pm *Manager) Cancel(pid int64) {
	pm.mutex.Lock()
	process, ok := pm.processes[pid]
	pm.mutex.Unlock()
	if ok {
		process.Cancel()
	}
}

// Processes gets the processes in a thread safe manner
func (pm *Manager) Processes(onlyRoots bool) []*Process {
	pm.mutex.Lock()
	processes := make([]*Process, 0, len(pm.processes))
	if onlyRoots {
		for _, process := range pm.processes {
			if process.ParentPID == 0 {
				processes = append(processes, process)
			}
		}
	} else {
		for _, process := range pm.processes {
			processes = append(processes, process)
		}
	}
	pm.mutex.Unlock()

	sort.Slice(processes, func(i, j int) bool {
		left, right := processes[i], processes[j]

		return left.Start.Before(right.Start)
	})

	return processes
}

// Exec a command and use the default timeout.
func (pm *Manager) Exec(desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDir(-1, "", desc, cmdName, args...)
}

// ExecTimeout a command and use a specific timeout duration.
func (pm *Manager) ExecTimeout(timeout time.Duration, desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDir(timeout, "", desc, cmdName, args...)
}

// ExecDir a command and use the default timeout.
func (pm *Manager) ExecDir(timeout time.Duration, dir, desc, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDirEnv(timeout, dir, desc, nil, cmdName, args...)
}

// ExecDirEnv runs a command in given path and environment variables, and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func (pm *Manager) ExecDirEnv(timeout time.Duration, dir, desc string, env []string, cmdName string, args ...string) (string, string, error) {
	return pm.ExecDirEnvStdIn(timeout, dir, desc, env, nil, cmdName, args...)
}

// ExecDirEnvStdIn runs a command in given path and environment variables with provided stdIN, and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func (pm *Manager) ExecDirEnvStdIn(timeout time.Duration, dir, desc string, env []string, stdIn io.Reader, cmdName string, args ...string) (string, string, error) {
	if timeout == -1 {
		timeout = 60 * time.Second
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	ctx, cancel := pm.AddContextTimeout(DefaultContext, timeout, desc)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if stdIn != nil {
		cmd.Stdin = stdIn
	}

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	err := cmd.Wait()

	if err != nil {
		err = &Error{
			PID:         GetPID(ctx),
			Description: desc,
			Err:         err,
			CtxErr:      ctx.Err(),
			Stdout:      stdOut.String(),
			Stderr:      stdErr.String(),
		}
	}

	return stdOut.String(), stdErr.String(), err
}

type processList []*Process

func (l processList) Len() int {
	return len(l)
}

func (l processList) Less(i, j int) bool {
	if l[i].ParentPID < l[j].ParentPID {
		return true
	}
	if l[i].ParentPID == l[j].ParentPID {
		return l[i].PID < l[j].PID
	}
	return false
}

func (l processList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Error is a wrapped error describing the error results of Process Execution
type Error struct {
	PID         int64
	Description string
	Err         error
	CtxErr      error
	Stdout      string
	Stderr      string
}

func (err *Error) Error() string {
	return fmt.Sprintf("exec(%d:%s) failed: %v(%v) stdout: %s stderr: %s", err.PID, err.Description, err.Err, err.CtxErr, err.Stdout, err.Stderr)
}

// Unwrap implements the unwrappable implicit interface for go1.13 Unwrap()
func (err *Error) Unwrap() error {
	return err.Err
}
