// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/templates"
	notify_service "code.gitea.io/gitea/services/notify"
	"code.gitea.io/gitea/services/pubsub"

	"github.com/olahol/melody"
)

type websocketNotifier struct {
	notify_service.NullNotifier
	m      *melody.Melody
	rnd    *templates.HTMLRender
	pubsub pubsub.Broker
}

// NewNotifier create a new webhooksNotifier notifier
func newNotifier(m *melody.Melody) notify_service.Notifier {
	return &websocketNotifier{
		m:   m,
		rnd: templates.HTMLRenderer(),
	}
}

// htmxAddElementEnd = "<div hx-swap-oob=\"beforebegin:%s\">%s</div>"
// htmxUpdateElement = "<div hx-swap-oob=\"outerHTML:%s\">%s</div>"

var htmxRemoveElement = "<div hx-swap-oob=\"delete:%s\"></div>"
