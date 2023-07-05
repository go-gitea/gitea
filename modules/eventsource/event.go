// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package eventsource

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
)

func wrapNewlines(w io.Writer, prefix, value []byte) (sum int64, err error) {
	if len(value) == 0 {
		return
	}
	var n int
	last := 0
	for j := bytes.IndexByte(value, '\n'); j > -1; j = bytes.IndexByte(value[last:], '\n') {
		n, err = w.Write(prefix)
		sum += int64(n)
		if err != nil {
			return
		}
		n, err = w.Write(value[last : last+j+1])
		sum += int64(n)
		if err != nil {
			return
		}
		last += j + 1
	}
	n, err = w.Write(prefix)
	sum += int64(n)
	if err != nil {
		return
	}
	n, err = w.Write(value[last:])
	sum += int64(n)
	if err != nil {
		return
	}
	n, err = w.Write([]byte("\n"))
	sum += int64(n)
	return sum, err
}

// Event is an eventsource event, not all fields need to be set
type Event struct {
	// Name represents the value of the event: tag in the stream
	Name string
	// Data is either JSONified []byte or any that can be JSONd
	Data any
	// ID represents the ID of an event
	ID string
	// Retry tells the receiver only to attempt to reconnect to the source after this time
	Retry time.Duration
}

// WriteTo writes data to w until there's no more data to write or when an error occurs.
// The return value n is the number of bytes written. Any error encountered during the write is also returned.
func (e *Event) WriteTo(w io.Writer) (int64, error) {
	sum := int64(0)
	var nint int
	n, err := wrapNewlines(w, []byte("event: "), []byte(e.Name))
	sum += n
	if err != nil {
		return sum, err
	}

	if e.Data != nil {
		var data []byte
		switch v := e.Data.(type) {
		case []byte:
			data = v
		case string:
			data = []byte(v)
		default:
			var err error
			data, err = json.Marshal(e.Data)
			if err != nil {
				return sum, err
			}
		}
		n, err := wrapNewlines(w, []byte("data: "), data)
		sum += n
		if err != nil {
			return sum, err
		}
	}

	n, err = wrapNewlines(w, []byte("id: "), []byte(e.ID))
	sum += n
	if err != nil {
		return sum, err
	}

	if e.Retry != 0 {
		nint, err = fmt.Fprintf(w, "retry: %d\n", int64(e.Retry/time.Millisecond))
		sum += int64(nint)
		if err != nil {
			return sum, err
		}
	}

	nint, err = w.Write([]byte("\n"))
	sum += int64(nint)

	return sum, err
}

func (e *Event) String() string {
	buf := new(strings.Builder)
	_, _ = e.WriteTo(buf)
	return buf.String()
}
