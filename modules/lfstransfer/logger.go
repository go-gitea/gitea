// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfstransfer

import (
	"github.com/charmbracelet/git-lfs-transfer/transfer"
)

var _ transfer.Logger = (*CodeNestLogger)(nil)

// noop logger for passing into transfer
type CodeNestLogger struct{}

func newLogger() transfer.Logger {
	return &CodeNestLogger{}
}

// Log implements transfer.Logger
func (g *CodeNestLogger) Log(msg string, items ...any) {
}
