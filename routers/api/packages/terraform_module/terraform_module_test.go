// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package terraform_module

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// packageName composes the {name}/{provider} tuple into the single
// package-name string stored by the underlying packages model. The
// encoding must be stable: callers that bypass this helper would
// fragment storage and silently produce 404s on download.
func TestPackageName(t *testing.T) {
	assert.Equal(t, "vpc/aws", packageName("vpc", "aws"))
	assert.Equal(t, "redis-cluster/gcp", packageName("redis-cluster", "gcp"))
	// The helper does not normalize case; callers are expected to have
	// already validated the inputs against tfmod.Validate* helpers,
	// which reject uppercase.
	assert.Equal(t, "VPC/AWS", packageName("VPC", "AWS"))
}
