// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadServerFromACMEProfile(t *testing.T) {
	defer test.MockVariableValue(&Protocol)()
	defer test.MockVariableValue(&EnableAcme)()
	defer test.MockVariableValue(&AcmeTOS)()
	defer test.MockVariableValue(&AcmeURL)()
	defer test.MockVariableValue(&AcmeProfile)()
	defer test.MockVariableValue(&AcmeCARoot)()
	defer test.MockVariableValue(&AcmeLiveDirectory)()
	defer test.MockVariableValue(&AcmeEmail)()

	cfg, err := NewConfigProviderFromData(`
[server]
PROTOCOL = https
ENABLE_ACME = true
ACME_ACCEPTTOS = true
ACME_DIRECTORY = /tmp/acme
ACME_EMAIL = acme@example.com
ACME_PROFILE = shortlived
`)
	assert.NoError(t, err)

	loadServerFrom(cfg)

	assert.Equal(t, HTTPS, Protocol)
	assert.Equal(t, "shortlived", AcmeProfile)
}
