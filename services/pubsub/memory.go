// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import (
	"context"
	"errors"
	"sync"
)

type Memory struct {
	sync.Mutex

	topics map[string]map[*Subscriber]struct{}
}

// New creates an in-memory publisher.
func NewMemory() Broker {
	return &Memory{
		topics: make(map[string]map[*Subscriber]struct{}),
	}
}

func (p *Memory) Publish(_ context.Context, _topic string, data []byte) error {
	p.Lock()

	topic, ok := p.topics[_topic]
	if !ok {
		p.Unlock()
		return errors.New("topic not found")
	}

	for s := range topic {
		go (*s)(data)
	}
	p.Unlock()

	return nil
}

func (p *Memory) Subscribe(c context.Context, topic string, subscriber Subscriber) {
	// Subscribe
	p.Lock()
	_, ok := p.topics[topic]
	if !ok {
		p.topics[topic] = make(map[*Subscriber]struct{})
	}
	p.topics[topic][&subscriber] = struct{}{}
	p.Unlock()

	// Wait for context to be done
	<-c.Done()

	// Unsubscribe
	p.Lock()
	delete(p.topics[topic], &subscriber)
	if len(p.topics[topic]) == 0 {
		delete(p.topics, topic)
	}
	p.Unlock()
}
