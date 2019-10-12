// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package task

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

var (
	_ Queue = &ChannelQueue{}
)

// ChannelQueue implements
type ChannelQueue struct {
	queue chan *models.Task
}

// NewChannelQueue create a memory channel queue
func NewChannelQueue(queueLen int) *ChannelQueue {
	return &ChannelQueue{
		queue: make(chan *models.Task, queueLen),
	}
}

// Run starts to run the queue
func (c *ChannelQueue) Run() error {
	for task := range c.queue {
		err := Run(task)
		if err != nil {
			log.Error("Run task failed: %s", err.Error())
		}
	}
	return nil
}

// Push will push the task ID to queue
func (c *ChannelQueue) Push(task *models.Task) error {
	c.queue <- task
	return nil
}

// Stop stop the queue
func (c *ChannelQueue) Stop() {
	close(c.queue)
}
