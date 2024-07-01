// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfstransfer

import (
	"code.gitea.io/gitea/modules/lfstransfer/transfer"
)

var _ transfer.Logger = (*GiteaLogger)(nil)

// noop logger for passing into transfer
type GiteaLogger struct{}

func newLogger() transfer.Logger {
	return &GiteaLogger{}
}

// Log implements transfer.Logger
func (g *GiteaLogger) Log(msg string, itms ...interface{}) {
}
