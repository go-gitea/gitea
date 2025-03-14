// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func listenReadAndClose(t *testing.T, l net.Listener, expected string) {
	conn, err := l.Accept()
	assert.NoError(t, err)
	defer conn.Close()
	written, err := io.ReadAll(conn)

	assert.NoError(t, err)
	assert.Equal(t, expected, string(written))
}

func TestConnLogger(t *testing.T) {
	protocol := "tcp"
	address := ":3099"

	l, err := net.Listen(protocol, address)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	prefix := "TestPrefix "
	level := INFO
	flags := LstdFlags | LUTC | Lfuncname

	logger := NewLoggerWithWriters(t.Context(), "test", NewEventWriterConn("test-conn", WriterMode{
		Level:        level,
		Prefix:       prefix,
		Flags:        FlagsFromBits(flags),
		WriterOption: WriterConnOption{Addr: address, Protocol: protocol, Reconnect: true, ReconnectOnMsg: true},
	}))

	location, _ := time.LoadLocation("EST")

	date := time.Date(2019, time.January, 13, 22, 3, 30, 15, location)

	dateString := date.UTC().Format("2006/01/02 15:04:05")

	event := Event{
		Level:         INFO,
		MsgSimpleText: "TEST MSG",
		Caller:        "CALLER",
		Filename:      "FULL/FILENAME",
		Line:          1,
		Time:          date,
	}
	expected := fmt.Sprintf("%s%s %s:%d:%s [%c] %s\n", prefix, dateString, event.Filename, event.Line, event.Caller, strings.ToUpper(event.Level.String())[0], event.MsgSimpleText)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		listenReadAndClose(t, l, expected)
	}()
	logger.SendLogEvent(&event)
	wg.Wait()

	logger.Close()
}
