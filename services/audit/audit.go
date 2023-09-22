// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util/rotatingfilewriter"
)

type TypeDescriptor struct {
	Type         string `json:"type"`
	PrimaryKey   any    `json:"primary_key"`
	FriendlyName string `json:"friendly_name"`
	Target       any    `json:"-"`
}

type Event struct {
	Action  Action         `json:"action"`
	Doer    TypeDescriptor `json:"doer"`
	Scope   TypeDescriptor `json:"scope"`
	Target  TypeDescriptor `json:"target"`
	Message string         `json:"message"`
	Time    time.Time      `json:"time"`
}

var (
	appenders  = make([]Appender, 0, 5)
	auditQueue *queue.WorkerPoolQueue[*Event]
)

func TestingOnlyAddAppender(a Appender) {
	appenders = append(appenders, a)
}

func TestingOnlyRemoveAppender(a Appender) {
	for i, app := range appenders {
		if app == a {
			last := len(appenders) - 1
			appenders[last], appenders[i] = nil, appenders[last]
			appenders = appenders[:last]
			return
		}
	}
}

func Init() {
	if !setting.Audit.Enabled {
		return
	}

	for name, opts := range setting.Audit.AppenderOptions {
		var a Appender
		switch name {
		case "log":
			a = &LogAppender{}
		case "file":
			if err := os.MkdirAll(filepath.Dir(opts.Filename), os.ModePerm); err != nil {
				panic(err.Error())
			}

			fa, err := NewFileAppender(opts.Filename, &rotatingfilewriter.Options{
				Rotate:           opts.Rotate,
				MaximumSize:      opts.MaximumSize,
				RotateDaily:      opts.RotateDaily,
				KeepDays:         opts.KeepDays,
				Compress:         opts.Compress,
				CompressionLevel: opts.CompressionLevel,
			})
			if err != nil {
				log.Error("Failed to create file appender: %v", err)
				continue
			}
			a = fa
		}

		if a != nil {
			appenders = append(appenders, a)
		}
	}

	auditQueue = queue.CreateSimpleQueue(
		graceful.GetManager().ShutdownContext(),
		"audit",
		func(data ...*Event) []*Event {
			ctx := graceful.GetManager().ShutdownContext()

			for _, e := range data {
				for _, a := range appenders {
					a.Record(ctx, e)
				}
			}
			return nil
		},
	)

	go graceful.GetManager().RunWithCancel(auditQueue)
}

func Record(action Action, doer *user_model.User, scope, target any, format string, v ...any) {
	if !setting.Audit.Enabled {
		return
	}

	e := BuildEvent(action, doer, scope, target, format, v...)

	if err := auditQueue.Push(e); err != nil {
		log.Error("Error pushing audit event to queue: %v", err)
	}
}

func BuildEvent(action Action, doer *user_model.User, scope, target any, format string, v ...any) *Event {
	return &Event{
		Action:  action,
		Doer:    typeToDescription(doer),
		Scope:   scopeToDescription(scope),
		Target:  typeToDescription(target),
		Message: fmt.Sprintf(format, v...),
		Time:    time.Now(),
	}
}

func scopeToDescription(scope any) TypeDescriptor {
	if scope == nil {
		return TypeDescriptor{"system", 0, "System", nil}
	}

	switch s := scope.(type) {
	case *repository_model.Repository, *user_model.User, *organization_model.Organization:
		return typeToDescription(scope)
	default:
		panic(fmt.Sprintf("unsupported scope type: %T", s))
	}
}

func typeToDescription(val any) TypeDescriptor {
	switch t := val.(type) {
	case *repository_model.Repository:
		return TypeDescriptor{"repository", t.ID, t.FullName(), val}
	case *user_model.User:
		if t.IsOrganization() {
			return TypeDescriptor{"organization", t.ID, t.Name, val}
		}
		return TypeDescriptor{"user", t.ID, t.Name, val}
	case *organization_model.Organization:
		return TypeDescriptor{"organization", t.ID, t.Name, val}
	case *user_model.EmailAddress:
		return TypeDescriptor{"email_address", t.ID, t.Email, val}
	case *organization_model.Team:
		return TypeDescriptor{"team", t.ID, t.Name, val}
	case *auth_model.TwoFactor:
		return TypeDescriptor{"twofactor", t.ID, "", val}
	case *auth_model.WebAuthnCredential:
		return TypeDescriptor{"webauthn", t.ID, t.Name, val}
	case *user_model.UserOpenID:
		return TypeDescriptor{"openid", t.ID, t.URI, val}
	case *auth_model.AccessToken:
		return TypeDescriptor{"access_token", t.ID, t.Name, val}
	case *auth_model.OAuth2Application:
		return TypeDescriptor{"oauth2_application", t.ID, t.Name, val}
	case *auth_model.OAuth2Grant:
		return TypeDescriptor{"oauth2_grant", t.ID, "", val}
	case *auth_model.Source:
		return TypeDescriptor{"authentication_source", t.ID, t.Name, val}
	case *user_model.ExternalLoginUser:
		return TypeDescriptor{"external_account", t.ExternalID, t.ExternalID, val}
	case *asymkey_model.PublicKey:
		return TypeDescriptor{"public_key", t.ID, t.Fingerprint, val}
	case *asymkey_model.GPGKey:
		return TypeDescriptor{"gpg_key", t.ID, t.KeyID, val}
	case *secret_model.Secret:
		return TypeDescriptor{"secret", t.ID, t.Name, val}
	case *webhook_model.Webhook:
		return TypeDescriptor{"webhook", t.ID, t.URL, val}
	case *git_model.ProtectedTag:
		return TypeDescriptor{"protected_tag", t.ID, t.NamePattern, val}
	case *git_model.ProtectedBranch:
		return TypeDescriptor{"protected_branch", t.ID, t.RuleName, val}
	case *repository_model.PushMirror:
		return TypeDescriptor{"push_mirror", t.ID, t.RemoteAddress, val}
	case *models.RepoTransfer:
		return TypeDescriptor{"repo_transfer", t.ID, "", val}
	default:
		panic(fmt.Sprintf("unsupported type: %T", t))
	}
}
