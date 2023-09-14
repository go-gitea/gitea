// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/json"

	saml2 "github.com/russellhaering/gosaml2"
)

//  _________   _____      _____  .____
// /   _____/  /  _  \    /     \ |    |
// \_____  \  /  /_\  \  /  \ /  \|    |
// /        \/    |    \/    Y    \    |___
///_______  /\____|__  /\____|__  /_______ \
//        \/         \/         \/        \/

// Source holds configuration for the SAML login source.
type Source struct {
	// IdentityProviderMetadata description: The SAML Identity Provider metadata XML contents (for static configuration of the SAML Service Provider). The value of this field should be an XML document whose root element is `<EntityDescriptor>` or `<EntityDescriptors>`. To escape the value into a JSON string, you may want to use a tool like https://json-escape-text.now.sh.
	IdentityProviderMetadata string
	// IdentityProviderMetadataURL description: The SAML Identity Provider metadata URL (for dynamic configuration of the SAML Service Provider).
	IdentityProviderMetadataURL string
	// InsecureSkipAssertionSignatureValidation description: Whether the Service Provider should (insecurely) accept assertions from the Identity Provider without a valid signature.
	InsecureSkipAssertionSignatureValidation bool
	// NameIDFormat description: The SAML NameID format to use when performing user authentication.
	NameIDFormat NameIDFormat
	// ServiceProviderCertificate description: The SAML Service Provider certificate in X.509 encoding (begins with "-----BEGIN CERTIFICATE-----"). This certificate is used by the Identity Provider to validate the Service Provider's AuthnRequests and LogoutRequests. It corresponds to the Service Provider's private key (`serviceProviderPrivateKey`). To escape the value into a JSON string, you may want to use a tool like https://json-escape-text.now.sh.
	ServiceProviderCertificate string
	// ServiceProviderIssuer description: The SAML Service Provider name, used to identify this Service Provider. This is required if the "externalURL" field is not set (as the SAML metadata endpoint is computed as "<externalURL>.auth/saml/metadata"), or when using multiple SAML authentication providers.
	ServiceProviderIssuer string
	// ServiceProviderPrivateKey description: The SAML Service Provider private key in PKCS#8 encoding (begins with "-----BEGIN PRIVATE KEY-----"). This private key is used to sign AuthnRequests and LogoutRequests. It corresponds to the Service Provider's certificate (`serviceProviderCertificate`). To escape the value into a JSON string, you may want to use a tool like https://json-escape-text.now.sh.
	ServiceProviderPrivateKey string
	// SignRequests description: Sign AuthnRequests and LogoutRequests sent to the Identity Provider using the Service Provider's private key (`serviceProviderPrivateKey`). It defaults to true if the `serviceProviderPrivateKey` and `serviceProviderCertificate` are set, and false otherwise.
	SignRequests bool

	CallbackURL string

	// EmailAssertionKey description: Assertion key for user.Email
	EmailAssertionKey string
	// NameAssertionKey description: Assertion key for user.NickName
	NameAssertionKey string
	// UsernameAssertionKey description: Assertion key for user.Name
	UsernameAssertionKey string

	// reference to the authSource
	authSource *auth.Source

	samlSP *saml2.SAMLServiceProvider
}

// FromDB fills up a SAML from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports a SAML to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

// SetAuthSource sets the related AuthSource
func (source *Source) SetAuthSource(authSource *auth.Source) {
	source.authSource = authSource
}

func init() {
	auth.RegisterTypeConfig(auth.SAML, &Source{})
}
