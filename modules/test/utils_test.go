// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupGiteaRoot(t *testing.T) {
	t.Setenv("GITEA_ROOT", "test")
	assert.Equal(t, "test", SetupGiteaRoot())
	t.Setenv("GITEA_ROOT", "")
	assert.NotEqual(t, "test", SetupGiteaRoot())
}
