// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/routers/web/auth"
	"github.com/olahol/melody"
)

type SessionData struct {
	unregister func()
}

func HandleConnect(s *melody.Session) {
	ctx := context.GetWebContext(s.Request)

	// Listen to connection close and un-register messageChan
	notify := ctx.Done()
	ctx.Resp.Flush()

	if !ctx.IsSigned {
		// Return unauthorized status event
		event := &eventsource.Event{
			Name: "close",
			Data: "unauthorized",
		}
		_, _ = event.WriteTo(ctx.Resp)
		ctx.Resp.Flush()
		return
	}

	shutdownCtx := graceful.GetManager().ShutdownContext()

	uid := ctx.Doer.ID

	messageChan := eventsource.GetManager().Register(uid)

	sessionData := &SessionData{
		unregister: func() {
			eventsource.GetManager().Unregister(uid, messageChan)
			// ensure the messageChan is closed
			for {
				_, ok := <-messageChan
				if !ok {
					break
				}
			}
		},
	}

	s.Set("data", sessionData)

	timer := time.NewTicker(30 * time.Second)

loop:
	for {
		select {
		case <-notify:
			go sessionData.unregister()
			break loop
		case <-shutdownCtx.Done():
			go sessionData.unregister()
			break loop
		case event, ok := <-messageChan:
			if !ok {
				break loop
			}

			// Handle logout
			if event.Name == "logout" {
				if ctx.Session.ID() == event.Data {
					_, _ = (&eventsource.Event{
						Name: "logout",
						Data: "here",
					}).WriteTo(ctx.Resp)
					ctx.Resp.Flush()
					go sessionData.unregister()
					auth.HandleSignOut(ctx)
					break loop
				}
				// Replace the event - we don't want to expose the session ID to the user
				event = &eventsource.Event{
					Name: "logout",
					Data: "elsewhere",
				}
			}

			_, err := event.WriteTo(ctx.Resp)
			if err != nil {
				log.Error("Unable to write to EventStream for user %s: %v", ctx.Doer.Name, err)
				go sessionData.unregister()
				break loop
			}
			ctx.Resp.Flush()
		}
	}
	timer.Stop()
}

func HandleMessage(s *melody.Session, msg []byte) {
	// TODO: Handle incoming messages
}

func getSessionData(s *melody.Session) (*SessionData, error) {
	_data, ok := s.Get("data")
	if !ok {
		return nil, fmt.Errorf("no session data")
	}

	data, ok := _data.(*SessionData)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	return data, nil
}

func HandleDisconnect(s *melody.Session) {
	data, err := getSessionData(s)
	if err != nil {
		log.Error("Unable to get session data: %v", err)
		return
	}

	data.unregister()
}
