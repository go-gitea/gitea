// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import "context"

// Message defines a published message.
type Message struct {
	// Data is the actual data in the entry.
	Data []byte `json:"data"`

	// Topic is the topic of the message.
	Topic string `json:"topic"`
}

// Subscriber receives published messages.
type Subscriber func(Message)

type Broker interface {
	Publish(c context.Context, message Message)
	Subscribe(c context.Context, topic string, subscriber Subscriber)
}
