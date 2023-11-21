// Copyright 2023 The Gitea Authors. All rights reserved.
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

	var bean any

	switch t {
	case audit_model.TypeRepository:
		bean = &repository_model.Repository{}
	case audit_model.TypeUser:
		bean = &user_model.User{}
	case audit_model.TypeOrganization:
		bean = &organization_model.Organization{}
	case audit_model.TypeEmailAddress:
		bean = &user_model.EmailAddress{}
	case audit_model.TypeTeam:
		bean = &organization_model.Team{}
	case audit_model.TypeWebAuthnCredential:
		bean = &auth_model.WebAuthnCredential{}
	case audit_model.TypeOpenID:
		bean = &user_model.UserOpenID{}
	case audit_model.TypeAccessToken:
		bean = &auth_model.AccessToken{}
	case audit_model.TypeOAuth2Application:
		bean = &auth_model.OAuth2Application{}
	case audit_model.TypeAuthenticationSource:
		bean = &auth_model.Source{}
	case audit_model.TypePublicKey:
		bean = &asymkey_model.PublicKey{}
	case audit_model.TypeGPGKey:
		bean = &asymkey_model.GPGKey{}
	case audit_model.TypeSecret:
		bean = &secret_model.Secret{}
	case audit_model.TypeWebhook:
		bean = &webhook_model.Webhook{}
	case audit_model.TypeProtectedTag:
		bean = &git_model.ProtectedTag{}
	case audit_model.TypeProtectedBranch:
		bean = &git_model.ProtectedBranch{}
	case audit_model.TypePushMirror:
		bean = &repository_model.PushMirror{}
	default:
		panic(fmt.Sprintf("unsupported type: %v", t))
	}

	if has, _ = db.GetByID(ctx, id, bean); !has {
		td = TypeDescriptor{t, id, nil}
	} else {
		td = typeToDescription(bean)
	}

	oc[id] = td

	return td
}
