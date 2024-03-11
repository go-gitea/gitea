// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/templates"
	notify_service "code.gitea.io/gitea/services/notify"
	"code.gitea.io/gitea/services/pubsub"
	"github.com/olahol/melody"
)

type websocketNotifier struct {
	notify_service.NullNotifier
	m   *melody.Melody
	rnd *templates.HTMLRender
}

// NewNotifier create a new webhooksNotifier notifier
func newNotifier(m *melody.Melody, pubsub pubsub.Broker) notify_service.Notifier {
	return &websocketNotifier{
		m:   m,
		rnd: templates.HTMLRenderer(),
	}
}

// htmxAddElementEnd = "<div hx-swap-oob=\"beforebegin:%s\">%s</div>"
// htmxUpdateElement = "<div hx-swap-oob=\"outerHTML:%s\">%s</div>"

var htmxRemoveElement = "<div hx-swap-oob=\"delete:%s\"></div>"

func (n *websocketNotifier) filterSessions(fn func(*melody.Session, *sessionData) bool) []*melody.Session {
	sessions, err := n.m.Sessions()
	if err != nil {
		log.Error("Failed to get sessions: %v", err)
		return nil
	}

	_sessions := make([]*melody.Session, 0, len(sessions))
	for _, s := range sessions {
		data, err := getSessionData(s)
		if err != nil {
			continue
		}

		if fn(s, data) {
			_sessions = append(_sessions, s)
		}
	}

	return _sessions
}
