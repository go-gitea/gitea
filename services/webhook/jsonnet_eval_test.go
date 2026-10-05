// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateJsonnetPackagistBody(t *testing.T) {
	snippet := `
local meta = std.extVar("meta");
local event_type = std.extVar("event_type");
local package_url = if std.objectHas(meta, "package_url") then meta.package_url else "";
if event_type == "push" then { repository: { url: package_url } } else {}
`
	out, err := evaluateJsonnet(snippet, "test.jsonnet", map[string]any{
		"meta":       map[string]any{"package_url": "https://packagist.org/packages/foo/bar"},
		"event_type": "push",
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{"repository":{"url":"https://packagist.org/packages/foo/bar"}}`, out)
}

func TestEvaluateJsonnetNatives(t *testing.T) {
	out, err := evaluateJsonnet(`std.native("gitea_sha256")("abc")`, "sha.jsonnet", nil)
	assert.NoError(t, err)
	assert.Contains(t, out, "ba7816bf")
}
