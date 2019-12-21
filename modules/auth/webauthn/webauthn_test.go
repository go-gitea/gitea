package webauthn

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"code.gitea.io/gitea/modules/setting"
)

func TestInit(t *testing.T) {
	Init()

	assert.Equal(t, setting.Domain, WebAuthn.Config.RPID)
	assert.Equal(t, setting.AppName, WebAuthn.Config.RPDisplayName)
	assert.Equal(t, setting.AppURL, WebAuthn.Config.RPOrigin)
}
