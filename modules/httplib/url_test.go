// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestIsRelativeURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	rel := []string{
		"",
		"foo",
		"/",
		"/foo?k=%20#abc",
	}
	for _, s := range rel {
		assert.True(t, IsRelativeURL(s), "rel = %q", s)
	}
	abs := []string{
		"//",
		"\\\\",
		"/\\",
		"\\/",
		"mailto:a@b.com",
		"https://test.com",
	}
	for _, s := range abs {
		assert.False(t, IsRelativeURL(s), "abs = %q", s)
	}
}

func TestIsCurrentGiteaSiteURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	good := []string{
		"?key=val",
		"/sub",
		"/sub/",
		"/sub/foo",
		"/sub/foo/",
		"http://localhost:3000/sub?key=val",
		"http://localhost:3000/sub/",
	}
	for _, s := range good {
		assert.True(t, IsCurrentGiteaSiteURL(s), "good = %q", s)
	}
	bad := []string{
		".",
		"foo",
		"/",
		"//",
		"\\\\",
		"/foo",
		"http://localhost:3000/sub/..",
		"http://localhost:3000/other",
		"http://other/",
	}
	for _, s := range bad {
		assert.False(t, IsCurrentGiteaSiteURL(s), "bad = %q", s)
	}

	setting.AppURL = "http://localhost:3000/"
	setting.AppSubURL = ""
	assert.False(t, IsCurrentGiteaSiteURL("//"))
	assert.False(t, IsCurrentGiteaSiteURL("\\\\"))
	assert.False(t, IsCurrentGiteaSiteURL("http://localhost"))
	assert.True(t, IsCurrentGiteaSiteURL("http://localhost:3000?key=val"))
}
