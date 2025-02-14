// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package log

import (
	"os"

	"github.com/mattn/go-isatty"
)

func init() {
	// when running gitea as a systemd unit with logging set to console, the output can not be colorized,
	// otherwise it spams the journal / syslog with escape sequences like "#033[0m#033[32mcmd/web.go:102:#033[32m"
	// this file covers non-windows platforms.
	CanColorStdout = isatty.IsTerminal(os.Stdout.Fd())
	CanColorStderr = isatty.IsTerminal(os.Stderr.Fd())
}
