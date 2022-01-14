// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webauthn

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	setting.Domain = "domain"
	setting.AppName = "AppName"
	setting.AppURL = "https://domain/"
	rpOrigin := "https://domain"

	Init()

	assert.Equal(t, setting.Domain, WebAuthn.Config.RPID)
	assert.Equal(t, setting.AppName, WebAuthn.Config.RPDisplayName)
	assert.Equal(t, rpOrigin, WebAuthn.Config.RPOrigin)
}
