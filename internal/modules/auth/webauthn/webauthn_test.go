// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webauthn

import (
	"testing"

	"code.gitea.io/gitea/internal/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	setting.Domain = "domain"
	setting.AppName = "AppName"
	setting.AppURL = "https://domain/"
	rpOrigin := []string{"https://domain"}

	Init()

	assert.Equal(t, setting.Domain, WebAuthn.Config.RPID)
	assert.Equal(t, setting.AppName, WebAuthn.Config.RPDisplayName)
	assert.Equal(t, rpOrigin, WebAuthn.Config.RPOrigins)
}
