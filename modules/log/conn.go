// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"io"
	"net"
)

type connWriter struct {
	innerWriter    io.WriteCloser
	ReconnectOnMsg bool   `json:"reconnectOnMsg"`
	Reconnect      bool   `json:"reconnect"`
	Net            string `json:"net"`
	Addr           string `json:"addr"`
}

// Close the inner writer
func (i *connWriter) Close() error {
	if i.innerWriter != nil {
		return i.innerWriter.Close()
	}
	return nil
}

// Write the data to the connection
func (i *connWriter) Write(p []byte) (int, error) {
	if i.neededConnectOnMsg() {
		if err := i.connect(); err != nil {
			return 0, err
		}
	}

	if i.ReconnectOnMsg {
		defer i.innerWriter.Close()
	}

	return i.innerWriter.Write(p)
}

func (i *connWriter) neededConnectOnMsg() bool {
	if i.Reconnect {
		i.Reconnect = false
		return true
	}

	if i.innerWriter == nil {
		return true
	}

	return i.ReconnectOnMsg
}

func (i *connWriter) connect() error {
	if i.innerWriter != nil {
		i.innerWriter.Close()
		i.innerWriter = nil
	}

	conn, err := net.Dial(i.Net, i.Addr)
	if err != nil {
		return err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		err = tcpConn.SetKeepAlive(true)
		if err != nil {
			return err
		}
	}

	i.innerWriter = conn
	return nil
}

// ConnLogger implements LoggerProvider.
// it writes messages in keep-live tcp connection.
type ConnLogger struct {
	WriterLogger
	ReconnectOnMsg bool   `json:"reconnectOnMsg"`
	Reconnect      bool   `json:"reconnect"`
	Net            string `json:"net"`
	Addr           string `json:"addr"`
}

// NewConn creates new ConnLogger returning as LoggerProvider.
func NewConn() LoggerProvider {
	conn := new(ConnLogger)
	conn.Level = TRACE
	return conn
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *ConnLogger) Init(jsonconfig string) error {
	err := json.Unmarshal([]byte(jsonconfig), log)
	if err != nil {
		return err
	}
	log.NewWriterLogger(&connWriter{
		ReconnectOnMsg: log.ReconnectOnMsg,
		Reconnect:      log.Reconnect,
		Net:            log.Net,
		Addr:           log.Addr,
	}, log.Level)
	return nil
}

// Flush does nothing for this implementation
func (log *ConnLogger) Flush() {
}

// GetName returns the default name for this implementation
func (log *ConnLogger) GetName() string {
	return "conn"
}

func init() {
	Register("conn", NewConn)
}
