// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

// CanColorStdout reports if we can color the Stdout
// Although we could do terminal sniffing and the like - in reality
// most tools on *nix are happy to display ansi colors.
// We will terminal sniff on Windows in console_windows.go
var CanColorStdout = true

// CanColorStderr reports if we can color the Stderr
var CanColorStderr = true
