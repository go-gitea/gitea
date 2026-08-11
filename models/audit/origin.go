// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

// Origin identifies how an audit event was initiated.
type Origin string

const (
	OriginUI  Origin = "ui"
	OriginAPI Origin = "api"
	OriginCLI Origin = "cli"
)
