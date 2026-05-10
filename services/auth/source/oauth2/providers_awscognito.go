// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

const ProviderNameAwsCognito = "aws-cognito"

// AwsCognitoProvider is a GothProvider for AWS Cognito (based on OpenID Connect)
type AwsCognitoProvider struct {
	OpenIDProvider
}

// Name provides the technical name for this provider
func (c *AwsCognitoProvider) Name() string {
	return ProviderNameAwsCognito
}

// DisplayName returns the friendly name for this provider
func (c *AwsCognitoProvider) DisplayName() string {
	return "AWS Cognito"
}

var _ GothProvider = &AwsCognitoProvider{}

func init() {
	RegisterGothProvider(&AwsCognitoProvider{})
}
