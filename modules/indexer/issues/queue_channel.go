// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// ChannelQueue implements
type ChannelQueue struct {
	queue       chan *IndexerData
	indexer     Indexer
	batchNumber int
}

// NewChannelQueue create a memory channel queue
func NewChannelQueue(indexer Indexer, batchNumber int) *ChannelQueue {
	return &ChannelQueue{
		queue:       make(chan *IndexerData, setting.Indexer.UpdateQueueLength),
		indexer:     indexer,
		batchNumber: batchNumber,
	}
}

// Run starts to run the queue
func (c *ChannelQueue) Run() error {
	var i int
	var datas = make([]*IndexerData, 0, c.batchNumber)
	for {
		select {
		case data := <-c.queue:
			if data.IsDelete {
				c.indexer.Delete(data.IDs...)
				continue
			}

			datas = append(datas, data)
			if len(datas) >= c.batchNumber {
				c.indexer.Index(datas)
				// TODO: save the point
				datas = make([]*IndexerData, 0, c.batchNumber)
			}
		case <-time.After(time.Millisecond * 100):
			i++
			if i >= 3 && len(datas) > 0 {
				c.indexer.Index(datas)
				// TODO: save the point
				datas = make([]*IndexerData, 0, c.batchNumber)
			}
		}
	}
}

// Push will push the indexer data to queue
func (c *ChannelQueue) Push(data *IndexerData) error {
	c.queue <- data
	return nil
}
