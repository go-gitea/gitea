// Copyright 2017 Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// GPGKey a user GPG key to sign commit and tag in repository
type GPGKey struct {
	// The unique identifier of the GPG key
	ID int64 `json:"id"`
	// The primary key ID of the GPG key
	PrimaryKeyID string `json:"primary_key_id"`
	// The key ID of the GPG key
	KeyID string `json:"key_id"`
	// The public key content in armored format
	PublicKey string `json:"public_key"`
	// List of email addresses associated with this GPG key
	Emails []*GPGKeyEmail `json:"emails"`
	// List of subkeys of this GPG key
	SubsKey []*GPGKey `json:"subkeys"`
	// Whether the key can be used for signing
	CanSign bool `json:"can_sign"`
	// Whether the key can be used for encrypting communications
	CanEncryptComms bool `json:"can_encrypt_comms"`
	// Whether the key can be used for encrypting storage
	CanEncryptStorage bool `json:"can_encrypt_storage"`
	// Whether the key can be used for certification
	CanCertify bool `json:"can_certify"`
	// Whether the GPG key has been verified
	Verified bool `json:"verified"`
	// swagger:strfmt date-time
	// The date and time when the GPG key was created
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	// The date and time when the GPG key expires
	Expires time.Time `json:"expires_at"`
}

// GPGKeyEmail an email attached to a GPGKey
// swagger:model GPGKeyEmail
type GPGKeyEmail struct {
	// The email address associated with the GPG key
	Email string `json:"email"`
	// Whether the email address has been verified
	Verified bool `json:"verified"`
}

// CreateGPGKeyOption options create user GPG key
type CreateGPGKeyOption struct {
	// An armored GPG key to add
	//
	// required: true
	// unique: true
	ArmoredKey string `json:"armored_public_key" binding:"Required"`
	// An optional armored signature for the GPG key
	Signature string `json:"armored_signature,omitempty"`
}

// VerifyGPGKeyOption options verifies user GPG key
type VerifyGPGKeyOption struct {
	// An Signature for a GPG key token
	//
	// required: true
	// The key ID of the GPG key to verify
	KeyID string `json:"key_id" binding:"Required"`
	// The armored signature to verify the GPG key
	Signature string `json:"armored_signature" binding:"Required"`
}
