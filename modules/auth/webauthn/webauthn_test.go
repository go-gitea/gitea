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
	Init()

	assert.Equal(t, setting.Domain, WebAuthn.Config.RPID)
	assert.Equal(t, setting.AppName, WebAuthn.Config.RPDisplayName)
	assert.Equal(t, setting.AppURL, WebAuthn.Config.RPOrigin)
}
