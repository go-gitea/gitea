// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
)

// ShutdownContext returns a context.Context that is Done at shutdown
// Callers using this context should ensure that they are registered as a running server
// in order that they are waited for.
func (g *Manager) ShutdownContext() context.Context {
	return g.shutdownCtx
}

// HammerContext returns a context.Context that is Done at hammer
// Callers using this context should ensure that they are registered as a running server
// in order that they are waited for.
func (g *Manager) HammerContext() context.Context {
	return g.hammerCtx
}

// TerminateContext returns a context.Context that is Done at terminate
// Callers using this context should ensure that they are registered as a terminating server
// in order that they are waited for.
func (g *Manager) TerminateContext() context.Context {
	return g.terminateCtx
}
