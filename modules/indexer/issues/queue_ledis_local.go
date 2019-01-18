// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"encoding/json"
	"time"

	"code.gitea.io/gitea/modules/log"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/ledis"
)

var (
	_ Queue = &LedisLocalQueue{}

	ledisLocalKey = []byte("ledis_local_key")
)

// LedisLocalQueue implements a ledis as a disk library queue
type LedisLocalQueue struct {
	indexer     Indexer
	ledis       *ledis.Ledis
	db          *ledis.DB
	batchNumber int
}

// NewLedisLocalQueue creates a ledis local queue
func NewLedisLocalQueue(indexer Indexer, dataDir string, dbIdx, batchNumber int) (*LedisLocalQueue, error) {
	ledis, err := ledis.Open(&config.Config{
		DataDir: dataDir,
	})
	if err != nil {
		return nil, err
	}

	db, err := ledis.Select(dbIdx)
	if err != nil {
		return nil, err
	}

	return &LedisLocalQueue{
		indexer:     indexer,
		ledis:       ledis,
		db:          db,
		batchNumber: batchNumber,
	}, nil
}

// Run starts to run the queue
func (l *LedisLocalQueue) Run() error {
	var i int
	var datas = make([]*IndexerData, 0, l.batchNumber)
	for {
		bs, err := l.db.RPop(ledisLocalKey)
		if err != nil {
			log.Error(4, "RPop: %v", err)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		i++
		if len(datas) > l.batchNumber || (len(datas) > 0 && i > 3) {
			l.indexer.Index(datas)
			datas = make([]*IndexerData, 0, l.batchNumber)
			i = 0
		}

		if len(bs) <= 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		var data IndexerData
		err = json.Unmarshal(bs, &data)
		if err != nil {
			log.Error(4, "Unmarshal: %v", err)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		log.Trace("LedisLocalQueue: task found: %#v", data)

		if data.IsDelete {
			if data.ID > 0 {
				if err = l.indexer.Delete(data.ID); err != nil {
					log.Error(4, "indexer.Delete: %v", err)
				}
			} else if len(data.IDs) > 0 {
				if err = l.indexer.Delete(data.IDs...); err != nil {
					log.Error(4, "indexer.Delete: %v", err)
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
func (l *LedisLocalQueue) Push(data *IndexerData) {
	bs, err := json.Marshal(data)
	if err != nil {
		log.Error(4, "Marshal: %v", err)
		return
	}
	_, err = l.db.LPush(ledisLocalKey, bs)
	if err != nil {
		log.Error(4, "LPush: %v", err)
	}
}
