// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	RunGitTests(m)
}

func TestParseGitVersion(t *testing.T) {
	v, err := parseGitVersionLine("git version 2.29.3")
	assert.NoError(t, err)
	assert.Equal(t, "2.29.3", v.String())

	v, err = parseGitVersionLine("git version 2.29.3.windows.1")
	assert.NoError(t, err)
	assert.Equal(t, "2.29.3", v.String())

	v, err = parseGitVersionLine("git version 2.28.0.618.gf4bc123cb7")
	assert.NoError(t, err)
	assert.Equal(t, "2.28.0", v.String())

	_, err = parseGitVersionLine("git version")
	assert.Error(t, err)

	_, err = parseGitVersionLine("git version windows")
	assert.Error(t, err)
}

func TestCheckGitVersionCompatibility(t *testing.T) {
	assert.NoError(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.0"))))
	assert.ErrorContains(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.1"))), "regression bug of GIT_FLUSH")
	assert.NoError(t, checkGitVersionCompatibility(version.Must(version.NewVersion("2.43.2"))))
}
