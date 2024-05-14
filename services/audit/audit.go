// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
)

type TypeDescriptor struct {
	Type   audit_model.ObjectType `json:"type"`
	ID     int64                  `json:"id"`
	Object any                    `json:"-"`
}

func (d TypeDescriptor) DisplayName() string {
	switch t := d.Object.(type) {
	case *repository_model.Repository:
		return t.FullName()
	case *user_model.User:
		return t.Name
	case *organization_model.Organization:
		return t.Name
	case *user_model.EmailAddress:
		return t.Email
	case *organization_model.Team:
		return t.Name
	case *auth_model.WebAuthnCredential:
		return t.Name
	case *user_model.UserOpenID:
		return t.URI
	case *auth_model.AccessToken:
		return t.Name
	case *auth_model.OAuth2Application:
		return t.Name
	case *auth_model.Source:
		return t.Name
	case *asymkey_model.PublicKey:
		return t.Fingerprint
	case *asymkey_model.GPGKey:
		return t.KeyID
	case *secret_model.Secret:
		return t.Name
	case *webhook_model.Webhook:
		return t.URL
	case *git_model.ProtectedTag:
		return t.NamePattern
	case *git_model.ProtectedBranch:
		return t.RuleName
	case *repository_model.PushMirror:
		return t.RemoteAddress
	}

	if d.Type == audit_model.TypeSystem {
		return "System"
	}

	return ""
}

func (d TypeDescriptor) HTMLURL() string {
	switch t := d.Object.(type) {
	case *repository_model.Repository:
		return t.HTMLURL()
	case *user_model.User:
		return t.HTMLURL()
	case *organization_model.Organization:
		return t.HTMLURL()
	}
	return ""
}

func Init() error {
	if !setting.Audit.Enabled {
		return nil
	}

	return initAuditFile()
}

var systemObject struct{}

func scopeToDescription(scope any) TypeDescriptor {
	if scope == &systemObject {
		return TypeDescriptor{audit_model.TypeSystem, 0, nil}
	}

	switch s := scope.(type) {
	case *repository_model.Repository, *user_model.User, *organization_model.Organization:
		return typeToDescription(scope)
	default:
		panic(fmt.Sprintf("unsupported scope type: %T", s))
	}
}

func typeToDescription(val any) TypeDescriptor {
	if val == &systemObject {
		return TypeDescriptor{audit_model.TypeSystem, 0, nil}
	}

	switch t := val.(type) {
	case *repository_model.Repository:
		return TypeDescriptor{audit_model.TypeRepository, t.ID, val}
	case *user_model.User:
		if t.IsOrganization() {
			return TypeDescriptor{audit_model.TypeOrganization, t.ID, val}
		}
		return TypeDescriptor{audit_model.TypeUser, t.ID, val}
	case *organization_model.Organization:
		return TypeDescriptor{audit_model.TypeOrganization, t.ID, val}
	case *user_model.EmailAddress:
		return TypeDescriptor{audit_model.TypeEmailAddress, t.ID, val}
	case *organization_model.Team:
		return TypeDescriptor{audit_model.TypeTeam, t.ID, val}
	case *auth_model.WebAuthnCredential:
		return TypeDescriptor{audit_model.TypeWebAuthnCredential, t.ID, val}
	case *user_model.UserOpenID:
		return TypeDescriptor{audit_model.TypeOpenID, t.ID, val}
	case *auth_model.AccessToken:
		return TypeDescriptor{audit_model.TypeAccessToken, t.ID, val}
	case *auth_model.OAuth2Application:
		return TypeDescriptor{audit_model.TypeOAuth2Application, t.ID, val}
	case *auth_model.Source:
		return TypeDescriptor{audit_model.TypeAuthenticationSource, t.ID, val}
	case *asymkey_model.PublicKey:
		return TypeDescriptor{audit_model.TypePublicKey, t.ID, val}
	case *asymkey_model.GPGKey:
		return TypeDescriptor{audit_model.TypeGPGKey, t.ID, val}
	case *secret_model.Secret:
		return TypeDescriptor{audit_model.TypeSecret, t.ID, val}
	case *webhook_model.Webhook:
		return TypeDescriptor{audit_model.TypeWebhook, t.ID, val}
	case *git_model.ProtectedTag:
		return TypeDescriptor{audit_model.TypeProtectedTag, t.ID, val}
	case *git_model.ProtectedBranch:
		return TypeDescriptor{audit_model.TypeProtectedBranch, t.ID, val}
	case *repository_model.PushMirror:
		return TypeDescriptor{audit_model.TypePushMirror, t.ID, val}
	default:
		panic(fmt.Sprintf("unsupported type: %T", t))
	}
}
