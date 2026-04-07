// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadAdminOrgDisabledFeatures(t *testing.T) {
	defer test.MockVariableValue(&Admin)()

	cfg, err := NewConfigProviderFromData(`
[admin]
ORG_DISABLED_FEATURES = danger_zone
`)
	assert.NoError(t, err)
	loadAdminFrom(cfg)

	assert.True(t, IsOrgFeatureDisabled(OrgFeatureDangerZone))
}

func TestLoadAdminOrgDisabledFeaturesDefault(t *testing.T) {
	defer test.MockVariableValue(&Admin)()

	cfg, err := NewConfigProviderFromData(`
[admin]
`)
	assert.NoError(t, err)
	loadAdminFrom(cfg)

	assert.False(t, IsOrgFeatureDisabled(OrgFeatureDangerZone))
}
