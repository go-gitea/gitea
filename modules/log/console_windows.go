// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/sys/windows"
)

func enableVTMode(console windows.Handle) bool {
	mode := uint32(0)
	err := windows.GetConsoleMode(console, &mode)
	if err != nil {
		return false
	}

	// EnableVirtualTerminalProcessing is the console mode to allow ANSI code
	// interpretation on the console. See:
	// https://docs.microsoft.com/en-us/windows/console/setconsolemode
	// It only works on windows 10. Earlier terminals will fail with an err which we will
	// handle to say don't color
	mode = mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	err = windows.SetConsoleMode(console, mode)
	return err == nil
}

func init() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		CanColorStdout = enableVTMode(windows.Stdout)
	} else {
		CanColorStdout = isatty.IsCygwinTerminal(os.Stderr.Fd())
	}

	if isatty.IsTerminal(os.Stderr.Fd()) {
		CanColorStderr = enableVTMode(windows.Stderr)
	} else {
		CanColorStderr = isatty.IsCygwinTerminal(os.Stderr.Fd())
	}
}
