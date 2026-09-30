// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadAcmeEABFrom(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&AcmeEABKID, ""))
	t.Cleanup(test.MockVariableValue(&AcmeEABHMAC, ""))

	cfg, err := NewConfigProviderFromData(`
[server]
ACME_EAB_KID = kid
ACME_EAB_HMAC = hmac
`)
	assert.NoError(t, err)

	loadAcmeEABFrom(cfg.Section("server"))

	assert.Equal(t, "kid", AcmeEABKID)
	assert.Equal(t, "hmac", AcmeEABHMAC)
}
