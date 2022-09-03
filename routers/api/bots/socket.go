// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/gorilla/websocket"
)

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
	BuildUUID    string // build uuid
	ErrCode      int    // error code
	ErrContent   string // errors message
	EventName    string
	EventPayload string
	JobID        string // only run the special job, empty means run all the jobs
}

const (
	version1 = 1
)

const (
	MsgTypeRegister       = iota + 1 // register
	MsgTypeError                     // error
	MsgTypeRequestBuild              // request build task
	MsgTypeIdle                      // no task
	MsgTypeBuildResult               // build result
	MsgTypeBuildJobResult            // build job result
)

func handleVersion1(r *http.Request, c *websocket.Conn, mt int, message []byte, msg *Message) error {
	switch msg.Type {
	case MsgTypeRegister:
		log.Info("websocket[%s] registered", r.RemoteAddr)
		runner, err := bots_model.GetRunnerByUUID(msg.RunnerUUID)
		if err != nil {
			if !errors.Is(err, bots_model.ErrRunnerNotExist{}) {
				return fmt.Errorf("websocket[%s] get runner [%s] failed: %v", r.RemoteAddr, msg.RunnerUUID, err)
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				return fmt.Errorf("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
			}
		} else {
			fmt.Printf("-----%v\n", runner)
			// TODO: handle read message
			err = c.WriteMessage(mt, message)
			if err != nil {
				return fmt.Errorf("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
			}
		}
	case MsgTypeRequestBuild:
		// TODO: find new task and send to client
		build, err := bots_model.GetCurBuildByUUID(msg.RunnerUUID)
		if err != nil {
			return fmt.Errorf("websocket[%s] get task[%s] failed: %v", r.RemoteAddr, msg.RunnerUUID, err)
		}
		var returnMsg *Message
		if build == nil {
			time.Sleep(3 * time.Second)
			returnMsg = &Message{
				Version:    version1,
				Type:       MsgTypeIdle,
				RunnerUUID: msg.RunnerUUID,
			}
		} else {
			returnMsg = &Message{
				Version:      version1,
				Type:         MsgTypeRequestBuild,
				RunnerUUID:   msg.RunnerUUID,
				BuildUUID:    build.UUID,
				EventName:    build.Event.Event(),
				EventPayload: build.EventPayload,
			}
		}
		bs, err := json.Marshal(&returnMsg)
		if err != nil {
			return fmt.Errorf("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
		}
		err = c.WriteMessage(mt, bs)
		if err != nil {
			return fmt.Errorf("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
		}
	case MsgTypeBuildResult:
		log.Info("websocket[%s] returned CI result: %v", r.RemoteAddr, msg)
		build, err := bots_model.GetBuildByUUID(msg.BuildUUID)
		if err != nil {
			return fmt.Errorf("websocket[%s] get build by uuid failed: %v", r.RemoteAddr, err)
		}
		cols := []string{"status", "end_time"}
		if msg.ErrCode == 0 {
			build.Status = core.StatusPassing
		} else {
			build.Status = core.StatusFailing
		}
		build.EndTime = timeutil.TimeStampNow()
		if err := bots_model.UpdateBuild(build, cols...); err != nil {
			log.Error("websocket[%s] update build failed: %v", r.RemoteAddr, err)
		}
	default:
		returnMsg := Message{
			Version:    version1,
			Type:       MsgTypeError,
			ErrCode:    1,
			ErrContent: fmt.Sprintf("message type %d is not supported", msg.Type),
		}
		bs, err := json.Marshal(&returnMsg)
		if err != nil {
			return fmt.Errorf("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
		}
		err = c.WriteMessage(mt, bs)
		if err != nil {
			return fmt.Errorf("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
		}
	}
	return nil
}

func socketServe(w http.ResponseWriter, r *http.Request) {
	log.Trace("websocket init request begin from %s", r.RemoteAddr)
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("websocket upgrade failed: %v", err)
		return
	}
	defer c.Close()
	log.Trace("websocket upgrade from %s successfully", r.RemoteAddr)

	_ = c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		// read message from client
		mt, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) ||
				websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				c.Close()
			} else if !strings.Contains(err.Error(), "i/o timeout") {
				log.Error("websocket[%s] read failed: %#v", r.RemoteAddr, err)
			}
			break
		}

		log.Trace("websocket[%s] received message: %s", r.RemoteAddr, string(message))

		// read message first
		var msg Message
		if err = json.Unmarshal(message, &msg); err != nil {
			log.Error("websocket[%s] unmarshal failed: %#v", r.RemoteAddr, err)
			break
		}

		switch msg.Version {
		case 1:
			if err := handleVersion1(r, c, mt, message, &msg); err != nil {
				log.Error("%v", err)
			}
		default:
			returnMsg := Message{
				Version:    1,
				Type:       MsgTypeError,
				ErrCode:    1,
				ErrContent: "version is not supported",
			}
			bs, err := json.Marshal(&returnMsg)
			if err != nil {
				log.Error("websocket[%s] marshal message failed: %v", r.RemoteAddr, err)
			} else {
				err = c.WriteMessage(mt, bs)
				if err != nil {
					log.Error("websocket[%s] sent message failed: %v", r.RemoteAddr, err)
				}
			}
		}
	}
}
