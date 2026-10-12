// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/mholt/acmez/v3/acme"
	"github.com/stretchr/testify/assert"
)

func TestAcmeExternalAccountBinding(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.AcmeEABKID, ""))
	t.Cleanup(test.MockVariableValue(&setting.AcmeEABHMAC, ""))

	binding, configured, err := acmeExternalAccountBinding()
	assert.NoError(t, err)
	assert.False(t, configured)
	assert.Empty(t, binding)

	setting.AcmeEABKID = "kid"
	_, _, err = acmeExternalAccountBinding()
	assert.ErrorContains(t, err, "both ACME_EAB_KID and ACME_EAB_HMAC must be set")

	setting.AcmeEABHMAC = "hmac"
	binding, configured, err = acmeExternalAccountBinding()
	assert.NoError(t, err)
	assert.True(t, configured)
	assert.Equal(t, acme.EAB{KeyID: "kid", MACKey: "hmac"}, binding)
}
