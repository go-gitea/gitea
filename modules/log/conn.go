// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"io"
	"log"
	"net"
)

// ConnWriter implements LoggerInterface.
// it writes messages in keep-live tcp connection.
type ConnWriter struct {
	lg             *log.Logger
	innerWriter    io.WriteCloser
	ReconnectOnMsg bool   `json:"reconnectOnMsg"`
	Reconnect      bool   `json:"reconnect"`
	Net            string `json:"net"`
	Addr           string `json:"addr"`
	Level          int    `json:"level"`
}

// NewConn creates new ConnWrite returning as LoggerInterface.
func NewConn() LoggerInterface {
	conn := new(ConnWriter)
	conn.Level = TRACE
	return conn
}

// Init inits connection writer with json config.
// json config only need key "level".
func (cw *ConnWriter) Init(jsonconfig string) error {
	return json.Unmarshal([]byte(jsonconfig), cw)
}

// WriteMsg writes message in connection.
// if connection is down, try to re-connect.
func (cw *ConnWriter) WriteMsg(msg string, skip, level int) error {
	if cw.Level > level {
		return nil
	}
	if cw.neededConnectOnMsg() {
		if err := cw.connect(); err != nil {
			return err
		}
	}

	if cw.ReconnectOnMsg {
		defer cw.innerWriter.Close()
	}
	cw.lg.Println(msg)
	return nil
}

// Flush no things for this implementation
func (cw *ConnWriter) Flush() {
}

// Destroy destroy connection writer and close tcp listener.
func (cw *ConnWriter) Destroy() {
	if cw.innerWriter == nil {
		return
	}
	cw.innerWriter.Close()
}

func (cw *ConnWriter) connect() error {
	if cw.innerWriter != nil {
		cw.innerWriter.Close()
		cw.innerWriter = nil
	}

	conn, err := net.Dial(cw.Net, cw.Addr)
	if err != nil {
		return err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
	}

	cw.innerWriter = conn
	cw.lg = log.New(conn, "", log.Ldate|log.Ltime)
	return nil
}

func (cw *ConnWriter) neededConnectOnMsg() bool {
	if cw.Reconnect {
		cw.Reconnect = false
		return true
	}

	if cw.innerWriter == nil {
		return true
	}

	return cw.ReconnectOnMsg
}

func init() {
	Register("conn", NewConn)
}
