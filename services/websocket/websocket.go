// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	gitea_context "code.gitea.io/gitea/services/context"
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
	URL string `json:"url"`
}

func Init() *melody.Melody {
	m = melody.New()
	hub := &hub{}
	m.HandleConnect(hub.handleConnect)
	m.HandleMessage(hub.handleMessage)
	m.HandleDisconnect(hub.handleDisconnect)

	broker := pubsub.InitWithNotifier()
	notifier := newNotifier(m)

	ctx, unsubscribe := context.WithCancel(context.Background())
	graceful.GetManager().RunAtShutdown(ctx, func() {
		unsubscribe()
	})

	broker.Subscribe(ctx, "notify", func(msg []byte) {
		data := struct {
			Function string
		}{}

		err := json.Unmarshal(msg, &data)
		if err != nil {
			log.Error("Failed to unmarshal message: %v", err)
			return
		}

		switch data.Function {
		case "DeleteComment":
			var data struct {
				Comment *issues_model.Comment
				Doer    *user_model.User
			}

			err := json.Unmarshal(msg, &data)
			if err != nil {
				log.Error("Failed to unmarshal message: %v", err)
				return
			}

			notifier.DeleteComment(context.Background(), data.Doer, data.Comment)
		}
	})

	return m
}

type hub struct{}

func (h *hub) handleConnect(s *melody.Session) {
	ctx := gitea_context.GetWebContext(s.Request)

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
		err := h.handleSubscribeMessage(data, msg.Data)
		if err != nil {
			return
		}
	}
}

func (h *hub) handleSubscribeMessage(data *sessionData, _data any) error {
	msgData := &subscribeMessageData{}
	err := mapstructure.Decode(_data, &msgData)
	if err != nil {
		return err
	}

	data.onURL = msgData.URL
	return nil
}

func (h *hub) handleDisconnect(s *melody.Session) {
}
