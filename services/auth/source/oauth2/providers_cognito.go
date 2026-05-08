// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

// CognitoProvider is a GothProvider for AWS Cognito (based on OpenID Connect)
type CognitoProvider struct {
	OpenIDProvider
}

// Name provides the technical name for this provider
func (c *CognitoProvider) Name() string {
	return "cognito"
}

// DisplayName returns the friendly name for this provider
func (c *CognitoProvider) DisplayName() string {
	return "AWS Cognito"
}

var _ GothProvider = &CognitoProvider{}

func init() {
	RegisterGothProvider(&CognitoProvider{})
}
