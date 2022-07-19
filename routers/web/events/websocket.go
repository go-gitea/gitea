// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package events

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/auth"
	"github.com/gorilla/websocket"
)

type readMessage struct {
	messageType int
	message     []byte
	err         error
}

// Events listens for events
func Websocket(ctx *context.Context) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header["Origin"]
			if len(origin) == 0 {
				return true
			}
			u, err := url.Parse(origin[0])
			if err != nil {
				return false
			}
			appURLURL, err := url.Parse(setting.AppURL)
			if err != nil {
				return true
			}

			return u.Host == appURLURL.Host
		},
	}

	// Because http proxies will tend not to pass these headers
	ctx.Req.Header.Add("Upgrade", "websocket")
	ctx.Req.Header.Add("Connection", "Upgrade")

	conn, err := upgrader.Upgrade(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("Unable to upgrade due to error: %v", err)
		return
	}
	defer conn.Close()

	notify := ctx.Done()
	shutdownCtx := graceful.GetManager().ShutdownContext()

	eventChan := make(<-chan *eventsource.Event)
	uid := int64(0)
	unregister := func() {}
	if ctx.IsSigned {
		uid = ctx.Doer.ID
		eventChan = eventsource.GetManager().Register(uid)
		unregister = func() {
			go func() {
				eventsource.GetManager().Unregister(uid, eventChan)
				// ensure the messageChan is closed
				for {
					_, ok := <-eventChan
					if !ok {
						break
					}
				}
			}()
		}
	}
	defer unregister()

	readChan := make(chan readMessage, 20)
	go func() {
		for {
			messageType, message, err := conn.ReadMessage()
			readChan <- readMessage{
				messageType: messageType,
				message:     message,
				err:         err,
			}
			if err != nil {
				close(readChan)
				return
			}
		}
	}()

	for {
		select {
		case <-notify:
			return
		case <-shutdownCtx.Done():
			return
		case _, ok := <-readChan:
			if !ok {
				break
			}
		case event, ok := <-eventChan:
			if !ok {
				break
			}
			if event.Name == "logout" {
				if ctx.Session.ID() == event.Data {
					_, _ = (&eventsource.Event{
						Name: "logout",
						Data: "here",
					}).WriteTo(ctx.Resp)
					ctx.Resp.Flush()
					go unregister()
					auth.HandleSignOut(ctx)
					break
				}
				// Replace the event - we don't want to expose the session ID to the user
				event = &eventsource.Event{
					Name: "logout",
					Data: "elsewhere",
				}
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Warn("Unable to get writer for websocket %v", err)
				return
			}

			if err := json.NewEncoder(w).Encode(event); err != nil {
				log.Error("Unable to create encoder for %v %v", event, err)
				return
			}
			if err := w.Close(); err != nil {
				log.Warn("Unable to close writer for websocket %v", err)
				return
			}

		}
	}
}
