// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/context"
	"github.com/olahol/melody"
)

func HandleConnect(s *melody.Session) {
	ctx := context.GetWebContext(s.Request)

	if !ctx.IsSigned {
		// Return unauthorized status event
		return
	}

	uid := ctx.Doer.ID
	sessionData := &sessionData{
		uid: uid,
	}
	s.Set("data", sessionData)

	// TODO: handle logouts
}

func HandleMessage(s *melody.Session, msg []byte) {
	// TODO: Handle incoming messages
}

func HandleDisconnect(s *melody.Session) {
	// TODO: Handle disconnect
}
