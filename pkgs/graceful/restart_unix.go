// +build !windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

package graceful

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

var killParent sync.Once

// KillParent sends the kill signal to the parent process if we are a child
func KillParent() {
	killParent.Do(func() {
		if GetManager().IsChild() {
			ppid := syscall.Getppid()
			if ppid > 1 {
				_ = syscall.Kill(ppid, syscall.SIGTERM)
			}
		}
	})
}

// RestartProcess starts a new process passing it the active listeners. It
// doesn't fork, but starts a new process using the same environment and
// arguments as when it was originally started. This allows for a newly
// deployed binary to be started. It returns the pid of the newly started
// process when successful.
func RestartProcess() (int, error) {
	listeners := getActiveListeners()

	// Extract the fds from the listeners.
	files := make([]*os.File, len(listeners))
	for i, l := range listeners {
		var err error
		// Now, all our listeners actually have File() functions so instead of
		// individually casting we just use a hacky interface
		files[i], err = l.(filer).File()
		if err != nil {
			return 0, err
		}

		if unixListener, ok := l.(*net.UnixListener); ok {
			unixListener.SetUnlinkOnClose(false)
		}
		// Remember to close these at the end.
		defer files[i].Close()
	}

	// Use the original binary location. This works with symlinks such that if
	// the file it points to has been changed we will use the updated symlink.
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return 0, err
	}

	// Pass on the environment and replace the old count key with the new one.
	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, listenFDs+"=") {
			env = append(env, v)
		}
	}
	env = append(env, fmt.Sprintf("%s=%d", listenFDs, len(listeners)))

	allFiles := append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...)
	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   env,
		Files: allFiles,
	})
	if err != nil {
		return 0, err
	}
	return process.Pid, nil
}
