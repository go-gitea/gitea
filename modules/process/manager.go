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

	processes map[IDType]*Process
}

// GetManager returns a Manager and initializes one as singleton if there's none yet
func GetManager() *Manager {
	managerInit.Do(func() {
		manager = &Manager{
			processes: make(map[IDType]*Process),
			next:      1,
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
	parentPID := GetParentPID(parent)

	ctx, cancel = context.WithCancel(parent)

	pid, finished := pm.Add(parentPID, description, cancel)

	return &Context{
		Context: ctx,
		pid:     pid,
	}, cancel, finished
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
	parentPID := GetParentPID(parent)

	ctx, cancel = context.WithTimeout(parent, timeout)

	pid, finshed := pm.Add(parentPID, description, cancel)

	return &Context{
		Context: ctx,
		pid:     pid,
	}, cancel, finshed
}

// Add create a new process
func (pm *Manager) Add(parentPID IDType, description string, cancel context.CancelFunc) (IDType, FinishedFunc) {
	pm.mutex.Lock()
	start, pid := pm.nextPID()

	parent := pm.processes[parentPID]
	if parent == nil {
		parentPID = ""
	}

	process := &Process{
		PID:         pid,
		ParentPID:   parentPID,
		Description: description,
		Start:       start,
		Cancel:      cancel,
	}

	finished := func() {
		cancel()
		pm.remove(process)
	}

	if parent != nil {
		parent.AddChild(process)
	}
	pm.processes[pid] = process
	pm.mutex.Unlock()

	return pid, finished
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
	delete(pm.processes, pid)
	pm.mutex.Unlock()
}

func (pm *Manager) remove(process *Process) {
	pm.mutex.Lock()
	if p := pm.processes[process.PID]; p == process {
		delete(pm.processes, process.PID)
	}
	parent := pm.processes[process.ParentPID]
	pm.mutex.Unlock()

	if parent == nil {
		return
	}

	parent.RemoveChild(process)
}

// Cancel a process in the ProcessManager.
func (pm *Manager) Cancel(pid IDType) {
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
			if _, has := pm.processes[process.ParentPID]; !has {
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

	ctx, _, finished := pm.AddContextTimeout(DefaultContext, timeout, desc)
	defer finished()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if stdIn != nil {
		cmd.Stdin = stdIn
	}
	SetSysProcAttribute(cmd)

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

// Error is a wrapped error describing the error results of Process Execution
type Error struct {
	PID         IDType
	Description string
	Err         error
	CtxErr      error
	Stdout      string
	Stderr      string
}

func (err *Error) Error() string {
	return fmt.Sprintf("exec(%s:%s) failed: %v(%v) stdout: %s stderr: %s", err.PID, err.Description, err.Err, err.CtxErr, err.Stdout, err.Stderr)
}

// Unwrap implements the unwrappable implicit interface for go1.13 Unwrap()
func (err *Error) Unwrap() error {
	return err.Err
}
