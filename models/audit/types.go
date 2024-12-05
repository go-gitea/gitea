// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

type ObjectType string

const (
	TypeSystem               ObjectType = "system"
	TypeRepository           ObjectType = "repository"
	TypeUser                 ObjectType = "user"
	TypeOrganization         ObjectType = "organization"
	TypeEmailAddress         ObjectType = "email_address"
	TypeTeam                 ObjectType = "team"
	TypeTwoFactor            ObjectType = "twofactor"
	TypeWebAuthnCredential   ObjectType = "webauthn"
	TypeOpenID               ObjectType = "openid"
	TypeAccessToken          ObjectType = "access_token"
	TypeOAuth2Application    ObjectType = "oauth2_application"
	TypeOAuth2Grant          ObjectType = "oauth2_grant"
	TypeAuthenticationSource ObjectType = "authentication_source"
	TypePublicKey            ObjectType = "public_key"
	TypeGPGKey               ObjectType = "gpg_key"
	TypeSecret               ObjectType = "secret"
	TypeWebhook              ObjectType = "webhook"
	TypeProtectedTag         ObjectType = "protected_tag"
	TypeProtectedBranch      ObjectType = "protected_branch"
	TypePushMirror           ObjectType = "push_mirror"
	TypeRepoTransfer         ObjectType = "repo_transfer"
)
