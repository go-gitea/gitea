// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"fmt"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
)

type cache = map[audit_model.ObjectType]map[int64]TypeDescriptor

func FindEvents(ctx context.Context, opts *audit_model.EventSearchOptions) ([]*Event, int64, error) {
	events, total, err := audit_model.FindEvents(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	return fromDatabaseEvents(ctx, events), total, nil
}

func fromDatabaseEvents(ctx context.Context, evs []*audit_model.Event) []*Event {
	c := cache{}

	users := make(map[int64]TypeDescriptor)
	for _, systemUser := range []*user_model.User{
		user_model.NewGhostUser(),
		user_model.NewActionsUser(),
		user_model.NewCLIUser(),
		user_model.NewAuthenticationSourceUser(),
	} {
		users[systemUser.ID] = typeToDescription(systemUser)
	}
	c[audit_model.TypeUser] = users

	events := make([]*Event, 0, len(evs))
	for _, e := range evs {
		events = append(events, fromDatabaseEvent(ctx, e, c))
	}
	return events
}

func fromDatabaseEvent(ctx context.Context, e *audit_model.Event, c cache) *Event {
	return &Event{
		Action:    e.Action,
		Actor:     resolveType(ctx, audit_model.TypeUser, e.ActorID, c),
		Scope:     resolveType(ctx, e.ScopeType, e.ScopeID, c),
		Target:    resolveType(ctx, e.TargetType, e.TargetID, c),
		Message:   e.Message,
		Time:      e.TimestampUnix.AsTime(),
		IPAddress: e.IPAddress,
	}
}

func resolveType(ctx context.Context, t audit_model.ObjectType, id int64, c cache) TypeDescriptor {
	oc, has := c[t]
	if !has {
		oc = make(map[int64]TypeDescriptor)
		c[t] = oc
	}

	td, has := oc[id]
	if has {
		return td
	}

	switch t {
	case audit_model.TypeSystem:
		td, has = typeToDescription(&systemObject), true
	case audit_model.TypeRepository:
		td, has = getTypeDescriptorByID[repository_model.Repository](ctx, id)
	case audit_model.TypeUser:
		td, has = getTypeDescriptorByID[user_model.User](ctx, id)
	case audit_model.TypeOrganization:
		td, has = getTypeDescriptorByID[organization_model.Organization](ctx, id)
	case audit_model.TypeEmailAddress:
		td, has = getTypeDescriptorByID[user_model.EmailAddress](ctx, id)
	case audit_model.TypeTeam:
		td, has = getTypeDescriptorByID[organization_model.Team](ctx, id)
	case audit_model.TypeWebAuthnCredential:
		td, has = getTypeDescriptorByID[auth_model.WebAuthnCredential](ctx, id)
	case audit_model.TypeOpenID:
		td, has = getTypeDescriptorByID[user_model.UserOpenID](ctx, id)
	case audit_model.TypeAccessToken:
		td, has = getTypeDescriptorByID[auth_model.AccessToken](ctx, id)
	case audit_model.TypeOAuth2Application:
		td, has = getTypeDescriptorByID[auth_model.OAuth2Application](ctx, id)
	case audit_model.TypeAuthenticationSource:
		td, has = getTypeDescriptorByID[auth_model.Source](ctx, id)
	case audit_model.TypePublicKey:
		td, has = getTypeDescriptorByID[asymkey_model.PublicKey](ctx, id)
	case audit_model.TypeGPGKey:
		td, has = getTypeDescriptorByID[asymkey_model.GPGKey](ctx, id)
	case audit_model.TypeSecret:
		td, has = getTypeDescriptorByID[secret_model.Secret](ctx, id)
	case audit_model.TypeWebhook:
		td, has = getTypeDescriptorByID[webhook_model.Webhook](ctx, id)
	case audit_model.TypeProtectedTag:
		td, has = getTypeDescriptorByID[git_model.ProtectedTag](ctx, id)
	case audit_model.TypeProtectedBranch:
		td, has = getTypeDescriptorByID[git_model.ProtectedBranch](ctx, id)
	case audit_model.TypePushMirror:
		td, has = getTypeDescriptorByID[repository_model.PushMirror](ctx, id)
	default:
		panic(fmt.Sprintf("unsupported type: %v", t))
	}

	if !has {
		td = TypeDescriptor{t, id, nil}
	}

	oc[id] = td

	return td
}

func getTypeDescriptorByID[T any](ctx context.Context, id int64) (TypeDescriptor, bool) {
	if bean, has, _ := db.GetByID[T](ctx, id); has {
		return typeToDescription(bean), true
	}

	return TypeDescriptor{}, false
}
