// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSecurityFrom(t *testing.T) {
	defer test.MockVariableValue(&Security)()

	assert.Equal(t, "SAMEORIGIN", Security.XFrameOptions)
	assert.Equal(t, "nosniff", Security.XContentTypeOptions)
	assert.Equal(t, "lax", Security.EgressMode)
	assert.Equal(t, "", Security.AllowedHostList)

	cfg, err := NewConfigProviderFromData(`[security]
X_FRAME_OPTIONS = DENY
X_CONTENT_TYPE_OPTIONS = unset
EGRESS_MODE = Strict
ALLOWED_HOST_LIST = foo
CONTENT_SECURITY_POLICY_GENERAL = "script-src *; foo"
`)
	assert.NoError(t, err)
	loadSecurityFrom(cfg)
	assert.Equal(t, "DENY", Security.XFrameOptions)
	assert.Equal(t, "unset", Security.XContentTypeOptions)
	assert.Equal(t, "strict", Security.EgressMode)
	assert.Equal(t, "foo", Security.AllowedHostList)
	assert.Equal(t, `"script-src *`, Security.ContentSecurityPolicyGeneral) // holy shit ini package bug
}

func TestLoadSecurityFromInternalRangesInvalid(t *testing.T) {
	defer test.MockVariableValue(&Security)()
	defer test.MockVariableValue(&StartupProblems)()
	cfg, err := NewConfigProviderFromData("[security]\nALLOWED_INTERNAL_RANGES = 10.0.0.5/8\n")
	require.NoError(t, err)
	loadSecurityFrom(cfg)
	assert.Equal(t, "10.0.0.5/8", Security.AllowedInternalRanges)
	found := false
	for _, problem := range StartupProblems {
		if strings.Contains(problem, "[security] ALLOWED_INTERNAL_RANGES") && strings.Contains(problem, "ignores an invalid entry") {
			found = true
		}
	}
	assert.True(t, found, "no startup problem for the invalid range: %v", StartupProblems)
}
