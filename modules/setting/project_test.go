// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadProjectDisableOrganizationProjects(t *testing.T) {
	defer test.MockVariableValue(&Project)()

	cfg, err := NewConfigProviderFromData(`
[project]
DISABLE_ORGANIZATION_PROJECTS = true
`)
	assert.NoError(t, err)

	loadProjectFrom(cfg)
	assert.True(t, Project.DisableOrganizationProjects)
}
