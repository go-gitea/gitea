// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// QueueSettings represent the settings for a queue from the ini
type QueueSettings struct {
	DataDir          string
	Length           int
	BatchLength      int
	ConnectionString string
	Type             string
	Network          string
	Addresses        string
	Password         string
	QueueName        string
	DBIndex          int
	WrapIfNecessary  bool
	MaxAttempts      int
	Timeout          time.Duration
	Workers          int
	MaxWorkers       int
	BlockTimeout     time.Duration
	BoostTimeout     time.Duration
	BoostWorkers     int
}

// Queue settings
var Queue = QueueSettings{}

// GetQueueSettings returns the queue settings for the appropriately named queue
func GetQueueSettings(name string) QueueSettings {
	q := QueueSettings{}
	sec := Cfg.Section("queue." + name)
	// DataDir is not directly inheritable
	q.DataDir = filepath.Join(Queue.DataDir, name)
	// QueueName is not directly inheritable either
	q.QueueName = name + Queue.QueueName
	for _, key := range sec.Keys() {
		switch key.Name() {
		case "DATADIR":
			q.DataDir = key.MustString(q.DataDir)
		case "QUEUE_NAME":
			q.QueueName = key.MustString(q.QueueName)
		}
	}
	if !filepath.IsAbs(q.DataDir) {
		q.DataDir = filepath.Join(AppDataPath, q.DataDir)
	}
	sec.Key("DATADIR").SetValue(q.DataDir)
	// The rest are...
	q.Length = sec.Key("LENGTH").MustInt(Queue.Length)
	q.BatchLength = sec.Key("BATCH_LENGTH").MustInt(Queue.BatchLength)
	q.ConnectionString = sec.Key("CONN_STR").MustString(Queue.ConnectionString)
	q.Type = sec.Key("TYPE").MustString(Queue.Type)
	q.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(Queue.WrapIfNecessary)
	q.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Queue.MaxAttempts)
	q.Timeout = sec.Key("TIMEOUT").MustDuration(Queue.Timeout)
	q.Workers = sec.Key("WORKERS").MustInt(Queue.Workers)
	q.MaxWorkers = sec.Key("MAX_WORKERS").MustInt(Queue.MaxWorkers)
	q.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(Queue.BlockTimeout)
	q.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(Queue.BoostTimeout)
	q.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(Queue.BoostWorkers)

	q.Network, q.Addresses, q.Password, q.DBIndex, _ = ParseQueueConnStr(q.ConnectionString)
	return q
}

// NewQueueService sets up the default settings for Queues
// This is exported for tests to be able to use the queue
func NewQueueService() {
	sec := Cfg.Section("queue")
	Queue.DataDir = sec.Key("DATADIR").MustString("queues/")
	if !filepath.IsAbs(Queue.DataDir) {
		Queue.DataDir = filepath.Join(AppDataPath, Queue.DataDir)
	}
	Queue.Length = sec.Key("LENGTH").MustInt(20)
	Queue.BatchLength = sec.Key("BATCH_LENGTH").MustInt(20)
	Queue.ConnectionString = sec.Key("CONN_STR").MustString(path.Join(AppDataPath, ""))
	Queue.Type = sec.Key("TYPE").MustString("")
	Queue.Network, Queue.Addresses, Queue.Password, Queue.DBIndex, _ = ParseQueueConnStr(Queue.ConnectionString)
	Queue.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(true)
	Queue.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(10)
	Queue.Timeout = sec.Key("TIMEOUT").MustDuration(GracefulHammerTime + 30*time.Second)
	Queue.Workers = sec.Key("WORKERS").MustInt(1)
	Queue.MaxWorkers = sec.Key("MAX_WORKERS").MustInt(10)
	Queue.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(1 * time.Second)
	Queue.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(5 * time.Minute)
	Queue.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(5)
	Queue.QueueName = sec.Key("QUEUE_NAME").MustString("_queue")

	// Now handle the old issue_indexer configuration
	section := Cfg.Section("queue.issue_indexer")
	issueIndexerSectionMap := map[string]string{}
	for _, key := range section.Keys() {
		issueIndexerSectionMap[key.Name()] = key.Value()
	}
	if _, ok := issueIndexerSectionMap["TYPE"]; !ok {
		switch Indexer.IssueQueueType {
		case LevelQueueType:
			section.Key("TYPE").SetValue("level")
		case ChannelQueueType:
			section.Key("TYPE").SetValue("persistable-channel")
		case RedisQueueType:
			section.Key("TYPE").SetValue("redis")
		default:
			log.Fatal("Unsupported indexer queue type: %v",
				Indexer.IssueQueueType)
		}
	}
	if _, ok := issueIndexerSectionMap["LENGTH"]; !ok {
		section.Key("LENGTH").SetValue(fmt.Sprintf("%d", Indexer.UpdateQueueLength))
	}
	if _, ok := issueIndexerSectionMap["BATCH_LENGTH"]; !ok {
		section.Key("BATCH_LENGTH").SetValue(fmt.Sprintf("%d", Indexer.IssueQueueBatchNumber))
	}
	if _, ok := issueIndexerSectionMap["DATADIR"]; !ok {
		section.Key("DATADIR").SetValue(Indexer.IssueQueueDir)
	}
	if _, ok := issueIndexerSectionMap["CONN_STR"]; !ok {
		section.Key("CONN_STR").SetValue(Indexer.IssueQueueConnStr)
	}
}

// ParseQueueConnStr parses a queue connection string
func ParseQueueConnStr(connStr string) (network, addrs, password string, dbIdx int, err error) {
	fields := strings.Fields(connStr)
	for _, f := range fields {
		items := strings.SplitN(f, "=", 2)
		if len(items) < 2 {
			continue
		}
		switch strings.ToLower(items[0]) {
		case "network":
			network = items[1]
		case "addrs":
			addrs = items[1]
		case "password":
			password = items[1]
		case "db":
			dbIdx, err = strconv.Atoi(items[1])
			if err != nil {
				return
			}
		}
	}
	return
}
