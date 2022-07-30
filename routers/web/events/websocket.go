// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package events

import (
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/web/auth"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	maximumMessageSize  = 2048
	readMessageChanSize = 20 // <- I've put 20 here because it seems like a reasonable buffer but it may to increase
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

	readMessageChan := make(chan readMessage, readMessageChanSize)
	go readMessagesFromConnToChan(conn, readMessageChan)

	pingTicker := time.NewTicker(pingPeriod)

	for {
		select {
		case <-notify:
			return
		case <-shutdownCtx.Done():
			return
		case <-pingTicker.C:
			// ensure that we're not already cancelled
			select {
			case <-notify:
				return
			case <-shutdownCtx.Done():
				return
			default:
			}

			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Error("unable to SetWriteDeadline: %v", err)
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error("unable to send PingMessage: %v", err)
				return
			}
		case message, ok := <-readMessageChan:
			if !ok {
				break
			}

			// ensure that we're not already cancelled
			select {
			case <-notify:
				return
			case <-shutdownCtx.Done():
				return
			default:
			}

			// FIXME: HANDLE MESSAGES
			log.Info("Got Message: %d:%s:%v", message.messageType, message.message, message.err)
		case event, ok := <-eventChan:
			if !ok {
				break
			}

			// ensure that we're not already cancelled
			select {
			case <-notify:
				return
			case <-shutdownCtx.Done():
				return
			default:
			}

			// Handle events
			if event.Name == "logout" {
				if ctx.Session.ID() == event.Data {
					event = &eventsource.Event{
						Name: "logout",
						Data: "here",
					}
					_ = writeEvent(conn, event)
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
			if err := writeEvent(conn, event); err != nil {
				return
			}
		}
	}
}

func readMessagesFromConnToChan(conn *websocket.Conn, messageChan chan readMessage) {
	defer func() {
		close(messageChan) // Please note: this has to be within a wrapping anonymous func otherwise it will be evaluated when creating the defer
		_ = conn.Close()
	}()
	conn.SetReadLimit(maximumMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Error("unable to SetReadDeadline: %v", err)
		return
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Error("unable to SetReadDeadline: %v", err)
		}
		return nil
	})

	for {
		messageType, message, err := conn.ReadMessage()
		messageChan <- readMessage{
			messageType: messageType,
			message:     message,
			err:         err,
		}
		if err != nil {
			// don't need to handle the error here as it is passed down the channel
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Error("unable to SetReadDeadline: %v", err)
			return
		}
	}
}

func writeEvent(conn *websocket.Conn, event *eventsource.Event) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		log.Error("unable to SetWriteDeadline: %v", err)
		return err
	}

	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		log.Error("Unable to get writer for websocket %v", err)
		return err
	}

	if err := json.NewEncoder(w).Encode(event); err != nil {
		log.Error("Unable to create encoder for %v %v", event, err)
		return err
	}
	if err := w.Close(); err != nil {
		log.Warn("Unable to close writer for websocket %v", err)
		return err
	}
	return nil
}
