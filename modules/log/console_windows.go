// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "golang.org/x/sys/windows"

const ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004

func enableVTMode(console windows.Handle) bool {
	mode := uint32(0)
	err := windows.GetConsoleMode(console, &mode)
	if err != nil {
		return false
	}
	mode = mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING
	err = windows.SetConsoleMode(console, mode)
	return err == nil
}

func init() {
	CanColorStdout = enableVTMode(windows.Stdout)
	CanColorStderr = enableVTMode(windows.Stderr)
}
