// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/log"

	ini "gopkg.in/ini.v1"
)

// QueueSettings represent the settings for a queue from the ini
type QueueSettings struct {
	Name             string
	DataDir          string
	QueueLength      int `ini:"LENGTH"`
	BatchLength      int
	ConnectionString string
	Type             string
	QueueName        string
	SetName          string
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
	q.Name = name

	// DataDir is not directly inheritable
	q.DataDir = filepath.ToSlash(filepath.Join(Queue.DataDir, "common"))
	// QueueName is not directly inheritable either
	q.QueueName = name + Queue.QueueName
	for _, key := range sec.Keys() {
		switch key.Name() {
		case "DATADIR":
			q.DataDir = key.MustString(q.DataDir)
		case "QUEUE_NAME":
			q.QueueName = key.MustString(q.QueueName)
		case "SET_NAME":
			q.SetName = key.MustString(q.SetName)
		}
	}
	if len(q.SetName) == 0 && len(Queue.SetName) > 0 {
		q.SetName = q.QueueName + Queue.SetName
	}
	if !filepath.IsAbs(q.DataDir) {
		q.DataDir = filepath.ToSlash(filepath.Join(AppDataPath, q.DataDir))
	}
	_, _ = sec.NewKey("DATADIR", q.DataDir)

	// The rest are...
	q.QueueLength = sec.Key("LENGTH").MustInt(Queue.QueueLength)
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

	return q
}

// NewQueueService sets up the default settings for Queues
// This is exported for tests to be able to use the queue
func NewQueueService() {
	sec := Cfg.Section("queue")
	Queue.DataDir = filepath.ToSlash(sec.Key("DATADIR").MustString("queues/"))
	if !filepath.IsAbs(Queue.DataDir) {
		Queue.DataDir = filepath.ToSlash(filepath.Join(AppDataPath, Queue.DataDir))
	}
	Queue.QueueLength = sec.Key("LENGTH").MustInt(20)
	Queue.BatchLength = sec.Key("BATCH_LENGTH").MustInt(20)
	Queue.ConnectionString = sec.Key("CONN_STR").MustString("")
	defaultType := sec.Key("TYPE").String()
	Queue.Type = sec.Key("TYPE").MustString("persistable-channel")
	Queue.WrapIfNecessary = sec.Key("WRAP_IF_NECESSARY").MustBool(true)
	Queue.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(10)
	Queue.Timeout = sec.Key("TIMEOUT").MustDuration(GracefulHammerTime + 30*time.Second)
	Queue.Workers = sec.Key("WORKERS").MustInt(0)
	Queue.MaxWorkers = sec.Key("MAX_WORKERS").MustInt(10)
	Queue.BlockTimeout = sec.Key("BLOCK_TIMEOUT").MustDuration(1 * time.Second)
	Queue.BoostTimeout = sec.Key("BOOST_TIMEOUT").MustDuration(5 * time.Minute)
	Queue.BoostWorkers = sec.Key("BOOST_WORKERS").MustInt(1)
	Queue.QueueName = sec.Key("QUEUE_NAME").MustString("_queue")
	Queue.SetName = sec.Key("SET_NAME").MustString("")

	// Now handle the old issue_indexer configuration
	// FIXME: DEPRECATED to be removed in v1.18.0
	section := Cfg.Section("queue.issue_indexer")
	directlySet := toDirectlySetKeysMap(section)
	if !directlySet["TYPE"] && defaultType == "" {
		switch typ := Cfg.Section("indexer").Key("ISSUE_INDEXER_QUEUE_TYPE").MustString(""); typ {
		case "levelqueue":
			_, _ = section.NewKey("TYPE", "level")
		case "channel":
			_, _ = section.NewKey("TYPE", "persistable-channel")
		case "redis":
			_, _ = section.NewKey("TYPE", "redis")
		case "":
			_, _ = section.NewKey("TYPE", "level")
		default:
			log.Fatal("Unsupported indexer queue type: %v", typ)
		}
	}
	if !directlySet["LENGTH"] {
		length := Cfg.Section("indexer").Key("UPDATE_BUFFER_LEN").MustInt(0)
		if length != 0 {
			_, _ = section.NewKey("LENGTH", strconv.Itoa(length))
		}
	}
	if !directlySet["BATCH_LENGTH"] {
		fallback := Cfg.Section("indexer").Key("ISSUE_INDEXER_QUEUE_BATCH_NUMBER").MustInt(0)
		if fallback != 0 {
			_, _ = section.NewKey("BATCH_LENGTH", strconv.Itoa(fallback))
		}
	}
	if !directlySet["DATADIR"] {
		queueDir := filepath.ToSlash(Cfg.Section("indexer").Key("ISSUE_INDEXER_QUEUE_DIR").MustString(""))
		if queueDir != "" {
			_, _ = section.NewKey("DATADIR", queueDir)
		}
	}
	if !directlySet["CONN_STR"] {
		connStr := Cfg.Section("indexer").Key("ISSUE_INDEXER_QUEUE_CONN_STR").MustString("")
		if connStr != "" {
			_, _ = section.NewKey("CONN_STR", connStr)
		}
	}

	// FIXME: DEPRECATED to be removed in v1.18.0
	// - will need to set default for [queue.*)] LENGTH appropriately though though

	// Handle the old mailer configuration
	handleOldLengthConfiguration("mailer", "mailer", "SEND_BUFFER_LEN", 100)

	// Handle the old test pull requests configuration
	// Please note this will be a unique queue
	handleOldLengthConfiguration("pr_patch_checker", "repository", "PULL_REQUEST_QUEUE_LENGTH", 1000)

	// Handle the old mirror queue configuration
	// Please note this will be a unique queue
	handleOldLengthConfiguration("mirror", "repository", "MIRROR_QUEUE_LENGTH", 1000)
}

// handleOldLengthConfiguration allows fallback to older configuration. `[queue.name]` `LENGTH` will override this configuration, but
// if that is left unset then we should fallback to the older configuration. (Except where the new length woul be <=0)
func handleOldLengthConfiguration(queueName, oldSection, oldKey string, defaultValue int) {
	if Cfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated fallback for %s queue length `[%s]` `%s` present. Use `[queue.%s]` `LENGTH`. This will be removed in v1.18.0", queueName, queueName, oldSection, oldKey)
	}
	value := Cfg.Section(oldSection).Key(oldKey).MustInt(defaultValue)

	// Don't override with 0
	if value <= 0 {
		return
	}

	section := Cfg.Section("queue." + queueName)
	directlySet := toDirectlySetKeysMap(section)
	if !directlySet["LENGTH"] {
		_, _ = section.NewKey("LENGTH", strconv.Itoa(value))
	}
}

// toDirectlySetKeysMap returns a bool map of keys directly set by this section
// Note: we cannot use section.HasKey(...) as that will immediately set the Key if a parent section has the Key
// but this section does not.
func toDirectlySetKeysMap(section *ini.Section) map[string]bool {
	sectionMap := map[string]bool{}
	for _, key := range section.Keys() {
		sectionMap[key.Name()] = true
	}
	return sectionMap
}
