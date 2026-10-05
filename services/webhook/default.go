// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"html/template"
	"net/http"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/svg"
	webhook_module "gitea.dev/modules/webhook"
)

type defaultHandler struct {
	gogs bool
}

func (h defaultHandler) Type() webhook_module.HookType {
	if h.gogs {
		return webhook_module.GOGS
	}
	return webhook_module.GITEA
}

func (h defaultHandler) DisplayName() string {
	if h.gogs {
		return "Gogs"
	}
	return "Gitea"
}

func (h defaultHandler) DocsURL() string {
	if h.gogs {
		return "https://gogs.io"
	}
	return "https://docs.gitea.com/usage/webhooks"
}

func (h defaultHandler) Icon(size int) template.HTML {
	if h.gogs {
		return imgIcon("gogs.png", size)
	}
	return svg.RenderHTML("gitea-gitea", size, "img")
}

func (h defaultHandler) FormFields() []FormField             { return nil }
func (h defaultHandler) Metadata(*webhook_model.Webhook) any { return nil }
func (h defaultHandler) RequiresPayloadURL() bool            { return true }
func (h defaultHandler) UseAuthorizationHeader() string      { return "optional" }
func (h defaultHandler) UseRequestSecret() string            { return "optional" }

func (h defaultHandler) NewRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	return newDefaultRequest(ctx, w, t)
}

func init() {
	RegisterHandler(defaultHandler{gogs: false})
	RegisterHandler(defaultHandler{gogs: true})
}
