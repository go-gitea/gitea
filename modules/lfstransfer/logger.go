package lfstransfer

import (
	"code.gitea.io/gitea/modules/lfstransfer/transfer"
)

// noop logger for passing into transfer
type GiteaLogger struct{}

// Log implements transfer.Logger
func (g *GiteaLogger) Log(msg string, itms ...interface{}) {
}

var _ transfer.Logger = (*GiteaLogger)(nil)

func newLogger() transfer.Logger {
	return &GiteaLogger{}
}
