// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// SAMLServiceProvider settings
var SAMLServiceProvider struct {
	RegisterEmailConfirm   bool
	EnableAutoRegistration bool
}

func loadSAMLServiceProviderFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("saml_service_provider")
	SAMLServiceProvider.RegisterEmailConfirm = sec.Key("REGISTER_EMAIL_CONFIRM").MustBool(Service.RegisterEmailConfirm)
	SAMLServiceProvider.EnableAutoRegistration = sec.Key("ENABLE_AUTO_REGISTRATION").MustBool()
}
