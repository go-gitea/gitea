// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tailmsg

import (
	"sync"
	"time"
)

type MsgRecord struct {
	Time    time.Time
	Content string
}

type MsgRecorder interface {
	Record(content string)
	GetRecords() []*MsgRecord
}

type memoryMsgRecorder struct {
	mu    sync.RWMutex
	msgs  []*MsgRecord
	limit int
}

// TODO: use redis for a clustered environment

func (m *memoryMsgRecorder) Record(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = append(m.msgs, &MsgRecord{
		Time:    time.Now(),
		Content: content,
	})
	if len(m.msgs) > m.limit {
		m.msgs = m.msgs[len(m.msgs)-m.limit:]
	}
}

func (m *memoryMsgRecorder) GetRecords() []*MsgRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ret := make([]*MsgRecord, len(m.msgs))
	copy(ret, m.msgs)
	return ret
}

func NewMsgRecorder(limit int) MsgRecorder {
	return &memoryMsgRecorder{
		limit: limit,
	}
}

type Manager struct {
	traceRecorder MsgRecorder
	logRecorder   MsgRecorder
}

func (m *Manager) GetTraceRecorder() MsgRecorder {
	return m.traceRecorder
}

func (m *Manager) GetLogRecorder() MsgRecorder {
	return m.logRecorder
}

var GetManager = sync.OnceValue(func() *Manager {
	return &Manager{
		traceRecorder: NewMsgRecorder(100),
		logRecorder:   NewMsgRecorder(1000),
	}
})
