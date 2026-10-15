// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webauthn

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	setting.AppDomain = "domain"
	setting.AppName = "AppName"
	setting.AppURL = "https://domain/"
	rpOrigin := []string{"https://domain"}

	Init()

	assert.Equal(t, setting.AppDomain, WebAuthn.Config.RPID)
	assert.Equal(t, setting.AppName, WebAuthn.Config.RPDisplayName)
	assert.Equal(t, rpOrigin, WebAuthn.Config.RPOrigins)
}

func TestInitUnsolicitedOutputPolicy(t *testing.T) {
	setting.AppDomain = "domain"
	setting.AppName = "AppName"
	setting.AppURL = "https://domain/"

	Init()

	// WebKit returns an unsolicited "appid" client extension output even when the
	// relying party did not request it, and go-webauthn v0.18 fails ceremonies over
	// such outputs under its default policy, so Gitea must opt out explicitly.
	assert.Equal(t, protocol.UnsolicitedOutputPolicyIgnore, WebAuthn.Config.ExtensionsUnsolicitedOutputPolicy)

	// The WebKit response shape must pass the extension validation the library
	// runs when finishing a login ceremony, driven with the policy Gitea configures.
	appID := false
	outputs := protocol.AuthenticationExtensionsClientOutputs{AppID: &appID}
	err := outputs.Verify(protocol.SessionExtensions{}, protocol.AssertCeremony, WebAuthn.Config.ExtensionsUnsolicitedOutputPolicy)
	assert.NoError(t, err)

	// Under the strict default (the zero value Gitea had before), the same WebKit
	// output fails the ceremony with the error users saw in production.
	err = outputs.Verify(protocol.SessionExtensions{}, protocol.AssertCeremony, protocol.UnsolicitedOutputPolicyReject)
	assert.ErrorContains(t, err, `Client returned the "appid" extension output which was not requested`)
}
