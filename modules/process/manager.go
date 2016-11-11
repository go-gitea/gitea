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

	"github.com/go-gitea/gitea/modules/log"
)

var (
	ErrExecTimeout = errors.New("Process execution timeout")
)

// Common timeout.
var (
	// NOTE: could be custom in config file for default.
	DEFAULT = 60 * time.Second
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

// Exec starts executing a command in given path, it records its process and timeout.
func ExecDir(timeout time.Duration, dir, desc, cmdName string, args ...string) (string, string, error) {
	if timeout == -1 {
		timeout = DEFAULT
	}

	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
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

// Exec starts executing a command, it records its process and timeout.
func ExecTimeout(timeout time.Duration, desc, cmdName string, args ...string) (string, string, error) {
	return ExecDir(timeout, "", desc, cmdName, args...)
}

// Exec starts executing a command, it records its process and has default timeout.
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
