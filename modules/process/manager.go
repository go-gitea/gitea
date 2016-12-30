// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// ErrExecTimeout represent a timeout error
	ErrExecTimeout = errors.New("Process execution timeout")

	// DefaultTimeout is the timeout used by Exec* family
	// of function when timeout parameter is omitted or
	// passed as -1
	// NOTE: could be custom in config file for default.
	DefaultTimeout = 60 * time.Second
)

// Process represents a working process inherit from Gogs.
type Process struct {
	Pid         int64 // Process ID, not system one.
	Description string
	Start       time.Time
	Cmd         *exec.Cmd
}

// List of existing processes.
var (
	curPid    int64 = 1
	Processes []*Process
)

// Add adds a existing process and returns its PID.
func Add(desc string, cmd *exec.Cmd) int64 {
	pid := curPid
	Processes = append(Processes, &Process{
		Pid:         pid,
		Description: desc,
		Start:       time.Now(),
		Cmd:         cmd,
	})
	curPid++
	return pid
}

// ExecDirEnv runs a command in given path and environment variables, and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func ExecDirEnv(timeout time.Duration, dir, desc string, env []string, cmdName string, args ...string) (string, string, error) {
	if timeout == -1 {
		timeout = DefaultTimeout
	}

	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = bufOut
	cmd.Stderr = bufErr
	if err := cmd.Start(); err != nil {
		return "", err.Error(), err
	}

	pid := Add(desc, cmd)
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	var err error
	select {
	case <-time.After(timeout):
		if errKill := Kill(pid); errKill != nil {
			log.Error(4, "Exec(%d:%s): %v", pid, desc, errKill)
		}
		<-done
		return "", ErrExecTimeout.Error(), ErrExecTimeout
	case err = <-done:
	}

	Remove(pid)
	return bufOut.String(), bufErr.String(), err
}

// ExecDir works exactly like ExecDirEnv except no environment variable is provided.
func ExecDir(timeout time.Duration, dir, desc, cmdName string, args ...string) (string, string, error) {
	return ExecDirEnv(timeout, dir, desc, nil, cmdName, args...)
}

// ExecTimeout runs a command and waits for its completion
// up to the given timeout (or DefaultTimeout if -1 is given).
// Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func ExecTimeout(timeout time.Duration, desc, cmdName string, args ...string) (string, string, error) {
	return ExecDir(timeout, "", desc, cmdName, args...)
}

// Exec runs a command and waits for its completion
// up to DefaultTimeout. Returns its complete stdout and stderr
// outputs and an error, if any (including timeout)
func Exec(desc, cmdName string, args ...string) (string, string, error) {
	return ExecDir(-1, "", desc, cmdName, args...)
}

// Remove removes a process from list.
func Remove(pid int64) {
	for i, proc := range Processes {
		if proc.Pid == pid {
			Processes = append(Processes[:i], Processes[i+1:]...)
			return
		}
	}
}

// Kill kills and removes a process from list.
func Kill(pid int64) error {
	for i, proc := range Processes {
		if proc.Pid == pid {
			if proc.Cmd != nil && proc.Cmd.Process != nil &&
				proc.Cmd.ProcessState != nil && !proc.Cmd.ProcessState.Exited() {
				if err := proc.Cmd.Process.Kill(); err != nil {
					return fmt.Errorf("fail to kill process(%d/%s): %v", proc.Pid, proc.Description, err)
				}
			}
			Processes = append(Processes[:i], Processes[i+1:]...)
			return nil
		}
	}
	return nil
}
