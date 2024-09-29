// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pubsub

import "context"

// Subscriber receives published messages.
type Subscriber func(data []byte)

type Broker interface {
	Publish(c context.Context, topic string, data []byte) error
	Subscribe(c context.Context, topic string, subscriber Subscriber)
}
