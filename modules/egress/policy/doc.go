// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package policy decides which outbound addresses Gitea may dial and provides
// the dialers and transports that enforce that decision.
//
// Every direct dial is checked at connect time (NewDialContext), so DNS rebinding
// cannot bypass the lists. Dials to a configured proxy are not checked — the proxy
// is operator configuration — but a direct request whose target is the proxy's own
// address is still screened.
//
// See design.md in this directory for the design overview and the decision records.
package policy
