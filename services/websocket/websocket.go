// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/context"

	"github.com/olahol/melody"
)

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

func HandleMessage(s *melody.Session, msg []byte) {
	data, err := getSessionData(s)
	if err != nil {
		return
	}

	// TODO: only handle specific url message
	data.onURL = string(msg)
}

func HandleDisconnect(s *melody.Session) {
	// TODO: Handle disconnect
}
