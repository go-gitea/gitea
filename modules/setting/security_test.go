// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadSecurityFrom(t *testing.T) {
	defer test.MockVariableValue(&Security)()

	// the package defaults, before any config is loaded
	assert.Equal(t, "SAMEORIGIN", Security.XFrameOptions)
	assert.Equal(t, "nosniff", Security.XContentTypeOptions)
	assert.Equal(t, "external", Security.AllowedHostList)

	testConfigLoad(t, []any{loadSecurityFrom}, []configTestCase{
		{
			name: "keys override the defaults",
			ini: `[security]
X_FRAME_OPTIONS = DENY
X_CONTENT_TYPE_OPTIONS = unset
ALLOWED_HOST_LIST = foo
CONTENT_SECURITY_POLICY_GENERAL = "script-src *; foo"
`,
			want: []configCheck{
				field("X_FRAME_OPTIONS", &Security.XFrameOptions, "DENY"),
				field("X_CONTENT_TYPE_OPTIONS", &Security.XContentTypeOptions, "unset"),
				field("ALLOWED_HOST_LIST", &Security.AllowedHostList, "foo"),
				field("CONTENT_SECURITY_POLICY_GENERAL", &Security.ContentSecurityPolicyGeneral, `"script-src *`), // holy shit ini package bug
			},
		},
	})
}
