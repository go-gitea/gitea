// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

type queueSettings struct {
	DataDir          string
	Length           int
	BatchLength      int
	ConnectionString string
	Type             string
	Addresses        string
	Password         string
	QueueName        string
	DBIndex          int
	WrapIfNecessary  bool
	MaxAttempts      int
	Timeout          time.Duration
	Workers          int
	BlockTimeout     time.Duration
	BoostTimeout     time.Duration
	BoostWorkers     int
}

// Queue settings
var Queue = queueSettings{}

// CreateQueue for name with provided handler and exemplar
func CreateQueue(name string, handle queue.HandlerFunc, exemplar interface{}) queue.Queue {
	q := getQueueSettings(name)
	opts := make(map[string]interface{})
	opts["Name"] = name
	opts["QueueLength"] = q.Length
	opts["BatchLength"] = q.BatchLength
	opts["DataDir"] = q.DataDir
	opts["Addresses"] = q.Addresses
	opts["Password"] = q.Password
	opts["DBIndex"] = q.DBIndex
	opts["QueueName"] = q.QueueName
	opts["Workers"] = q.Workers
	opts["BlockTimeout"] = q.BlockTimeout
	opts["BoostTimeout"] = q.BoostTimeout
	opts["BoostWorkers"] = q.BoostWorkers

	cfg, err := json.Marshal(opts)
	if err != nil {
		log.Error("Unable to marshall generic options: %v Error: %v", opts, err)
		log.Error("Unable to create queue for %s", name, err)
		return nil
	}

	returnable, err := queue.CreateQueue(queue.Type(q.Type), handle, cfg, exemplar)
	if q.WrapIfNecessary && err != nil {
		log.Warn("Unable to create queue for %s: %v", name, err)
		log.Warn("Attempting to create wrapped queue")
		returnable, err = queue.CreateQueue(queue.WrappedQueueType, handle, queue.WrappedQueueConfiguration{
			Underlying:  queue.Type(q.Type),
			Timeout:     q.Timeout,
			MaxAttempts: q.MaxAttempts,
			Config:      cfg,
			QueueLength: q.Length,
		}, exemplar)
	}
	if err != nil {
		log.Error("Unable to create queue for %s: %v", name, err)
		return nil
	}
	return returnable
}

func getQueueSettings(name string) queueSettings {
	q := queueSettings{}
	sec := Cfg.Section("queue." + name)
	// DataDir is not directly inheritable
	q.DataDir = path.Join(Queue.DataDir, name)
	for _, key := range sec.Keys() {
		switch key.Name() {
		case "DATADIR":
			q.DataDir = key.MustString(q.DataDir)
		}
	}
	if !path.IsAbs(q.DataDir) {
		q.DataDir = path.Join(AppDataPath, q.DataDir)
	}
	sec.Key("DATADIR").SetValue(q.DataDir)
	// The rest are...
	q.Length = sec.Key("LENGTH").MustInt(Queue.Length)
	q.BatchLength = sec.Key("BATCH_LENGTH").MustInt(Queue.BatchLength)
	q.ConnectionString = sec.Key("CONN_STR").MustString(Queue.ConnectionString)
	validTypes := queue.RegisteredTypesAsString()
	q.Type = sec.Key("TYPE").In(Queue.Type, validTypes)
	q.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(Queue.WrapIfNecessary)
	q.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Queue.MaxAttempts)
	q.Timeout = sec.Key("TIMEOUT").MustDuration(Queue.Timeout)
	q.Workers = sec.Key("WORKERS").MustInt(Queue.Workers)
	q.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(Queue.BlockTimeout)
	q.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(Queue.BoostTimeout)
	q.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(Queue.BoostWorkers)
	q.QueueName = sec.Key("QUEUE_NAME").MustString(Queue.QueueName)

	q.Addresses, q.Password, q.DBIndex, _ = ParseQueueConnStr(q.ConnectionString)
	return q
}

// NewQueueService sets up the default settings for Queues
// This is exported for tests to be able to use the queue
func NewQueueService() {
	sec := Cfg.Section("queue")
	Queue.DataDir = sec.Key("DATADIR").MustString("queues/")
	if !path.IsAbs(Queue.DataDir) {
		Queue.DataDir = path.Join(AppDataPath, Queue.DataDir)
	}
	Queue.Length = sec.Key("LENGTH").MustInt(20)
	Queue.BatchLength = sec.Key("BATCH_LENGTH").MustInt(20)
	Queue.ConnectionString = sec.Key("CONN_STR").MustString(path.Join(AppDataPath, ""))
	validTypes := queue.RegisteredTypesAsString()
	Queue.Type = sec.Key("TYPE").In(string(queue.PersistableChannelQueueType), validTypes)
	Queue.Addresses, Queue.Password, Queue.DBIndex, _ = ParseQueueConnStr(Queue.ConnectionString)
	Queue.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(true)
	Queue.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(10)
	Queue.Timeout = sec.Key("TIMEOUT").MustDuration(GracefulHammerTime + 30*time.Second)
	Queue.Workers = sec.Key("WORKERS").MustInt(1)
	Queue.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(1 * time.Second)
	Queue.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(5 * time.Minute)
	Queue.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(5)
	Queue.QueueName = sec.Key("QUEUE_NAME").MustString(Queue.QueueName)

	hasWorkers := false
	for _, key := range Cfg.Section("queue.notification").Keys() {
		if key.Name() == "WORKERS" {
			hasWorkers = true
			break
		}
	}
	if !hasWorkers {
		Cfg.Section("queue.notification").Key("WORKERS").SetValue("5")
	}

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
func ParseQueueConnStr(connStr string) (addrs, password string, dbIdx int, err error) {
	fields := strings.Fields(connStr)
	for _, f := range fields {
		items := strings.SplitN(f, "=", 2)
		if len(items) < 2 {
			continue
		}
		switch strings.ToLower(items[0]) {
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
