// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/sys/windows"
)

// EnableVirtualTerminalProcessing is the console mode to allow ANSI code
// interpretation on the console. SeeL
// https://docs.microsoft.com/en-us/windows/console/setconsolemode
const EnableVirtualTerminalProcessing = 0x0004

func enableVTMode(console windows.Handle) bool {
	mode := uint32(0)
	err := windows.GetConsoleMode(console, &mode)
	if err != nil {
		return false
	}
	mode = mode | EnableVirtualTerminalProcessing
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
