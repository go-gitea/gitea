// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"encoding/json"
	"time"

	"code.gitea.io/gitea/modules/log"
	"github.com/lunny/levelqueue"
)

var (
	_ Queue = &LevelQueue{}
)

// LevelQueue implements a disk library queue
type LevelQueue struct {
	indexer     Indexer
	queue       *levelqueue.Queue
	batchNumber int
}

// NewLevelQueue creates a ledis local queue
func NewLevelQueue(indexer Indexer, dataDir string, batchNumber int) (*LevelQueue, error) {
	queue, err := levelqueue.Open(dataDir)
	if err != nil {
		return nil, err
	}

	return &LevelQueue{
		indexer:     indexer,
		queue:       queue,
		batchNumber: batchNumber,
	}, nil
}

// Run starts to run the queue
func (l *LevelQueue) Run() error {
	var i int
	var datas = make([]*IndexerData, 0, l.batchNumber)
	for {
		i++
		if len(datas) > l.batchNumber || (len(datas) > 0 && i > 3) {
			_ = l.indexer.Index(datas)
			datas = make([]*IndexerData, 0, l.batchNumber)
			i = 0
			continue
		}

		bs, err := l.queue.RPop()
		if err != nil {
			if err != levelqueue.ErrNotFound {
				log.Error("RPop: %v", err)
			}
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if len(bs) == 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		var data IndexerData
		err = json.Unmarshal(bs, &data)
		if err != nil {
			log.Error("Unmarshal: %v", err)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		log.Trace("LevelQueue: task found: %#v", data)

		if data.IsDelete {
			if data.ID > 0 {
				if err = l.indexer.Delete(data.ID); err != nil {
					log.Error("indexer.Delete: %v", err)
				}
			} else if len(data.IDs) > 0 {
				if err = l.indexer.Delete(data.IDs...); err != nil {
					log.Error("indexer.Delete: %v", err)
				}
			}
			time.Sleep(time.Millisecond * 10)
			continue
		}

		datas = append(datas, &data)
		time.Sleep(time.Millisecond * 10)
	}
}

// Push will push the indexer data to queue
func (l *LevelQueue) Push(data *IndexerData) error {
	bs, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return l.queue.LPush(bs)
}
