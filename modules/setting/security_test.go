// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadSecurityFrom(t *testing.T) {
	assert.Equal(t, "SAMEORIGIN", Security.XFrameOptions)
	assert.Equal(t, "nosniff", Security.XContentTypeOptions)
	assert.Equal(t, "external", Security.AllowedHostList)

	cfg, err := NewConfigProviderFromData(`[security]
X_FRAME_OPTIONS = DENY
X_CONTENT_TYPE_OPTIONS = unset
ALLOWED_HOST_LIST = foo
CONTENT_SECURITY_POLICY_GENERAL = "script-src *; foo"
`)
	assert.NoError(t, err)
	loadSecurityFrom(cfg)
	assert.Equal(t, "DENY", Security.XFrameOptions)
	assert.Equal(t, "unset", Security.XContentTypeOptions)
	assert.Equal(t, "foo", Security.AllowedHostList)
	assert.Equal(t, `"script-src *`, Security.ContentSecurityPolicyGeneral) // holy shit ini package bug
}
