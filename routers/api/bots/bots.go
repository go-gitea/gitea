// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"

	"github.com/gorilla/websocket"
)

func Routes() *web.Route {
	r := web.NewRoute()
	r.Get("/", Serve)
	return r
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var pongWait = 60 * time.Second

type Message struct {
	Version      int    //
	Type         int    // message type, 1 register 2 error 3 task 4 no task
	RunnerUUID   string // runner uuid
	ErrCode      int    // error code
	ErrContent   string // errors message
	EventName    string
	EventPayload string
}

func Serve(w http.ResponseWriter, r *http.Request) {
	log.Trace("websocket init request begin from %s", r.RemoteAddr)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("websocket upgrade failed: %v", err)
		return
	}
	defer c.Close()
	log.Trace("websocket upgrade from %s successfully", r.RemoteAddr)

	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error { c.SetReadDeadline(time.Now().Add(pongWait)); return nil })

MESSAGE_BUMP:
	for {
		// read log from client
		mt, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) ||
				websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.Close()
				break
			}
			if !strings.Contains(err.Error(), "i/o timeout") {
				log.Error("websocket[%s] read failed: %#v", r.RemoteAddr, err)
			}
			break
		} else {
			log.Trace("websocket[%s] received message: %s", r.RemoteAddr, message)
		}

		// read message first
		var msg Message
		if err = json.Unmarshal(message, &msg); err != nil {
			log.Error("websocket[%s] unmarshal failed: %#v", r.RemoteAddr, err)
			break
		}

		switch msg.Version {
		case 1:
			switch msg.Type {
			case 1:
				log.Info("websocket[%s] registered", r.RemoteAddr)
				runner, err := bots_model.GetRunnerByUUID(msg.RunnerUUID)
				if err != nil {
					if !errors.Is(err, bots_model.ErrRunnerNotExist{}) {
						log.Error("websocket[%s] get runner [%s] failed: %v", r.RemoteAddr, msg.RunnerUUID, err)
						break
					}
					err = c.WriteMessage(mt, message)
					if err != nil {
						log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
						break
					}
				} else {
					fmt.Printf("-----%v\n", runner)
					// TODO: handle read message
					err = c.WriteMessage(mt, message)
					if err != nil {
						log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
						break
					}
				}
			default:
				returnMsg := Message{
					Version:    1,
					Type:       2,
					ErrCode:    1,
					ErrContent: fmt.Sprintf("message type %d is not supported", msg.Type),
				}
				bs, err := json.Marshal(&returnMsg)
				if err != nil {
					log.Error("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
					break MESSAGE_BUMP
				}
				err = c.WriteMessage(mt, bs)
				if err != nil {
					log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
				}
				break MESSAGE_BUMP
			}
		default:
			returnMsg := Message{
				Version:    1,
				Type:       2,
				ErrCode:    1,
				ErrContent: "version is not supported",
			}
			bs, err := json.Marshal(&returnMsg)
			if err != nil {
				log.Error("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
				break MESSAGE_BUMP
			}
			err = c.WriteMessage(mt, bs)
			if err != nil {
				log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
			}
			break MESSAGE_BUMP
		}

		// TODO: find new task and send to client
		task, err := bots_model.GetCurBuildByUUID(msg.RunnerUUID)
		if err != nil {
			log.Error("websocket[%s] get task failed: %v", r.RemoteAddr, err)
			break
		}
		if task == nil {
			returnMsg := Message{
				Version: 1,
				Type:    4,
			}
			bs, err := json.Marshal(&returnMsg)
			if err != nil {
				log.Error("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
				break MESSAGE_BUMP
			}
			err = c.WriteMessage(mt, bs)
			if err != nil {
				log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
			}
		} else {
			returnMsg := Message{
				Version:      1,
				Type:         3,
				EventName:    task.Event.Event(),
				EventPayload: task.EventPayload,
			}
			bs, err := json.Marshal(&returnMsg)
			if err != nil {
				log.Error("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
				break
			}
			err = c.WriteMessage(mt, bs)
			if err != nil {
				log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
			}
		}

	}
}
