// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/setting"
)

// IsEnrollmentEnabled reports whether enrollment via /api/v1/agents/enroll is enabled.
func IsEnrollmentEnabled(ctx context.Context) bool {
	return setting.Config().Agent.EnrollmentEnabled.Value(ctx)
}

// IsEnrollmentRemoteAllowed checks whether a remote address is allowed to enroll.
func IsEnrollmentRemoteAllowed(remoteAddr, allowList string) bool {
	allowList = strings.TrimSpace(allowList)
	if allowList == "" {
		return true
	}
	return hostmatcher.ParseHostMatchList("agent.enrollment.allowed_cidrs", allowList).MatchHostName(remoteAddr)
}

// IsEnrollmentRequestAllowed checks remote address against configured enrollment allow list.
func IsEnrollmentRequestAllowed(ctx context.Context, remoteAddr string) bool {
	return IsEnrollmentRemoteAllowed(remoteAddr, setting.Config().Agent.EnrollmentAllowedCIDRs.Value(ctx))
}
