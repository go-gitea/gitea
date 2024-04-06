// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	goContext "context"
	"fmt"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	notify_service "code.gitea.io/gitea/services/notify"
	"code.gitea.io/gitea/services/pubsub"

	"github.com/mitchellh/mapstructure"
	"github.com/olahol/melody"
)

var m *melody.Melody

type websocketMessage struct {
	Action string `json:"action"`
	Data   any    `json:"data"`
}

type subscribeMessageData struct {
	Repo string `json:"repo"`
}

func Init() *melody.Melody {
	m = melody.New()
	hub := &hub{
		pubsub: pubsub.NewMemory(),
	}
	m.HandleConnect(hub.handleConnect)
	m.HandleMessage(hub.handleMessage)
	m.HandleDisconnect(handleDisconnect)
	notify_service.RegisterNotifier(newNotifier(m))
	return m
}

type hub struct {
	pubsub pubsub.Broker
}

func (h *hub) handleConnect(s *melody.Session) {
	ctx := context.GetWebContext(s.Request)

	data := &sessionData{}
	if ctx.IsSigned {
		data.user = ctx.Doer
	}

	s.Set("data", data)

	// TODO: handle logouts
}

func (h *hub) handleMessage(s *melody.Session, _msg []byte) {
	data, err := getSessionData(s)
	if err != nil {
		return
	}

	msg := &websocketMessage{}
	err = json.Unmarshal(_msg, msg)
	if err != nil {
		return
	}

	switch msg.Action {
	case "subscribe":
		err := h.handleSubscribeMessage(s, data, msg.Data)
		if err != nil {
			return
		}
	}
}

func (h *hub) handleSubscribeMessage(s *melody.Session, data *sessionData, _data any) error {
	msgData := &subscribeMessageData{}
	err := mapstructure.Decode(_data, &msgData)
	if err != nil {
		return err
	}

	ctx := goContext.Background() // TODO: use proper context
	h.pubsub.Subscribe(ctx, msgData.Repo, func(msg pubsub.Message) {
		if data.user != nil {
			return
		}

		// TODO: check permissions
		hasAccess, err := access.HasAccessUnit(ctx, data.user, repo, unit.TypeIssues, perm.AccessModeRead)
		if err != nil {
			log.Error("Failed to check access: %v", err)
			return
		}

		if !hasAccess {
			return
		}

		// TODO: check the actual data received from pubsub and send it correctly to the client
		d := fmt.Sprintf(htmxRemoveElement, fmt.Sprintf("#%s", c.HashTag()))
		_ = s.Write([]byte(d))
	})

	return nil
}

func handleDisconnect(s *melody.Session) {
}
