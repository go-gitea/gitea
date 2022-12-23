// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package eventsource

import "sync"

// Messenger is a per uid message store
type Messenger struct {
	mutex    sync.Mutex
	uid      int64
	channels []chan *Event
}

// NewMessenger creates a messenger for a particular uid
func NewMessenger(uid int64) *Messenger {
	return &Messenger{
		uid:      uid,
		channels: [](chan *Event){},
	}
}

// Register returns a new chan []byte
func (m *Messenger) Register() <-chan *Event {
	m.mutex.Lock()
	// TODO: Limit the number of messengers per uid
	channel := make(chan *Event, 1)
	m.channels = append(m.channels, channel)
	m.mutex.Unlock()
	return channel
}

// Unregister removes the provider chan []byte
func (m *Messenger) Unregister(channel <-chan *Event) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i, toRemove := range m.channels {
		if channel == toRemove {
			m.channels = append(m.channels[:i], m.channels[i+1:]...)
			close(toRemove)
			break
		}
	}
	return len(m.channels) == 0
}

// UnregisterAll removes all chan []byte
func (m *Messenger) UnregisterAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, channel := range m.channels {
		close(channel)
	}
	m.channels = nil
}

// SendMessage sends the message to all registered channels
func (m *Messenger) SendMessage(message *Event) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i := range m.channels {
		channel := m.channels[i]
		select {
		case channel <- message:
		default:
		}
	}
}

// SendMessageBlocking sends the message to all registered channels and ensures it gets sent
func (m *Messenger) SendMessageBlocking(message *Event) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i := range m.channels {
		m.channels[i] <- message
	}
}
