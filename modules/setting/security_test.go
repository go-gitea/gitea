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

func TestWebhookInternalRangesWideningWarning(t *testing.T) {
	defer test.MockVariableValue(&Security)()
	defer test.MockVariableValue(&Webhook)()
	defer test.MockVariableValue(&StartupProblems)()

	cfg, err := NewConfigProviderFromData(`[security]
ALLOWED_INTERNAL_RANGES = 10.0.0.0/8
[webhook]
ALLOWED_HOST_LIST = ci.internal.example.com
`)
	require.NoError(t, err)
	loadSecurityFrom(cfg)
	loadWebhookFrom(cfg)
	assert.Equal(t, "ci.internal.example.com", Webhook.AllowedHostList)

	// the widening warning covers the effective webhook list, not just the security one
	found := false
	for _, problem := range StartupProblems {
		if strings.Contains(problem, "[webhook] ALLOWED_HOST_LIST") && strings.Contains(problem, "may now reach non-public targets") {
			found = true
		}
	}
	assert.True(t, found, "no widening warning for the webhook list: %v", StartupProblems)
}
