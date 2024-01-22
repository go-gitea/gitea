// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

type NameIDFormat int

const (
	SAML11Email NameIDFormat = iota + 1
	SAML11Persistent
	SAML11Unspecified
	SAML20Email
	SAML20Persistent
	SAML20Transient
	SAML20Unspecified
)

const DefaultNameIDFormat NameIDFormat = SAML20Persistent

var NameIDFormatNames = map[NameIDFormat]string{
	SAML11Email:       "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
	SAML11Persistent:  "urn:oasis:names:tc:SAML:1.1:nameid-format:persistent",
	SAML11Unspecified: "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
	SAML20Email:       "urn:oasis:names:tc:SAML:2.0:nameid-format:emailAddress",
	SAML20Persistent:  "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent",
	SAML20Transient:   "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
	SAML20Unspecified: "urn:oasis:names:tc:SAML:2.0:nameid-format:unspecified",
}

// String returns the name of the NameIDFormat
func (n NameIDFormat) String() string {
	return NameIDFormatNames[n]
}

// Int returns the int value of the NameIDFormat
func (n NameIDFormat) Int() int {
	return int(n)
}
