// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"fmt"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"

	"github.com/olahol/melody"
)

type websocketMessage struct {
	Action string `json:"action"`
	Data   string `json:"data"`
}

type subscribeMessageData struct {
	URL string `json:"url"`
}

func HandleConnect(s *melody.Session) {
	ctx := context.GetWebContext(s.Request)

	data := &sessionData{}

	if ctx.IsSigned {
		data.isSigned = true
		data.userID = ctx.Doer.ID
	}

	s.Set("data", data)

	// TODO: handle logouts
}

func HandleMessage(s *melody.Session, _msg []byte) {
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

func HandleDisconnect(s *melody.Session) {
	// TODO: Handle disconnect
}
