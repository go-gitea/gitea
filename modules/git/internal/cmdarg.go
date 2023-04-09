// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

// CmdArg represents a command argument for git command, and it will be used for the git command directly without any further processing.
// In most cases, you should use the "AddXxx" functions to add arguments, but not use this type directly.
// Casting a risky (user-provided) string to CmdArg would cause security issues if it's injected with a "--xxx" argument.
type CmdArg string
