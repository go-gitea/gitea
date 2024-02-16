// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"fmt"

	"github.com/olahol/melody"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	notify_service "code.gitea.io/gitea/services/notify"
)

var m *melody.Melody

type websocketMessage struct {
	Action string `json:"action"`
	Data   string `json:"data"`
}

type subscribeMessageData struct {
	URL string `json:"url"`
}

func Init() *melody.Melody {
	m = melody.New()
	m.HandleConnect(handleConnect)
	m.HandleMessage(handleMessage)
	m.HandleDisconnect(handleDisconnect)
	notify_service.RegisterNotifier(newNotifier(m))
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
	msgData, ok := _data.(*subscribeMessageData)
	if !ok {
		return fmt.Errorf("invalid message data")
	}

	data.onURL = msgData.URL
	return nil
}

func handleDisconnect(s *melody.Session) {
}
