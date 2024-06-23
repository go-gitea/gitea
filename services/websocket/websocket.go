// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/json"
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
	URL string `json:"url"`
}

func Init() *melody.Melody {
	m = melody.New()
	m.HandleConnect(handleConnect)
	m.HandleMessage(handleMessage)
	m.HandleDisconnect(handleDisconnect)

	broker := pubsub.NewMemory() // TODO: allow for other pubsub implementations
	notify_service.RegisterNotifier(newNotifier(m, broker))
	return m
}

func handleConnect(s *melody.Session) {
	ctx := context.GetWebContext(s.Request)

	data := &sessionData{}
	if ctx.IsSigned {
		data.user = ctx.Doer
	}

	s.Set("data", data)

	// TODO: handle logouts
}

func handleMessage(s *melody.Session, _msg []byte) {
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
		err := handleSubscribeMessage(data, msg.Data)
		if err != nil {
			return
		}
	}
}

func handleSubscribeMessage(data *sessionData, _data any) error {
	msgData := &subscribeMessageData{}
	err := mapstructure.Decode(_data, &msgData)
	if err != nil {
		return err
	}

	data.onURL = msgData.URL
	return nil
}

func handleDisconnect(s *melody.Session) {
}
