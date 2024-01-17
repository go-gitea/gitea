// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"context"
)

// Shutdown procedure:
// * cancel ShutdownContext: the registered context consumers have time to do their cleanup (they could use the hammer context)
// * cancel HammerContext: the all context consumers have limited time to do their cleanup (wait for a few seconds)
// * cancel TerminateContext: the registered context consumers have time to do their cleanup (but they shouldn't use shutdown/hammer context anymore)
// * cancel manager context
// If the shutdown is triggered again during the shutdown procedure, the hammer context will be canceled immediately to force to shut down.

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
