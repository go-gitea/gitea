// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSharedWorker(t *testing.T) {
	RegisterEventWriter("dummy", func(writerName string, writerMode WriterMode) EventWriter {
		return newDummyWriter(writerName, writerMode.Level, 0)
	})

	m := NewManager()
	_, err := m.NewSharedWriter("dummy-1", "dummy", WriterMode{Level: DEBUG, Flags: FlagsFromBits(0)})
	assert.NoError(t, err)

	w := m.GetSharedWriter("dummy-1")
	assert.NotNil(t, w)
	loggerTest := m.GetLogger("test")
	loggerTest.AddWriters(w)
	loggerTest.Info("msg-1")
	loggerTest.ReplaceAllWriters() // the shared writer is not closed here
	loggerTest.Info("never seen")

	// the shared writer can still be used later
	w = m.GetSharedWriter("dummy-1")
	assert.NotNil(t, w)
	loggerTest.AddWriters(w)
	loggerTest.Info("msg-2")

	m.GetLogger("test-another").AddWriters(w)
	m.GetLogger("test-another").Info("msg-3")

	m.Close()

	logs := w.(*dummyWriter).GetLogs()
	assert.Equal(t, []string{"msg-1\n", "msg-2\n", "msg-3\n"}, logs)
}
