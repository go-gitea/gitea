// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
)

// QueueSettings represent the settings for a queue from the ini
type QueueSettings struct {
	Name string // not an INI option, it is the name for [queue.the-name] section

	Type    string
	Datadir string
	ConnStr string // for levelqueue or redis
	Length  int    // queue length

	QueueName, SetName string

	BatchLength int
	MaxWorkers  int
}

var queueSettingsDefault = QueueSettings{
	Type:    "level",   // dummy, channel, level(leveldb,levelqueue,persistable-channel), redis
	Datadir: "queues/", // relative to AppDataPath
	Length:  20,        // queue length before a channel queue will block

	QueueName:   "_queue",  // the suffix of the default redis/disk queue name
	SetName:     "_unique", // the suffix of the default redis/disk unique queue set name (for unique queue)
	BatchLength: 20,
	MaxWorkers:  10,
}

func GetQueueSettings(rootCfg ConfigProvider, name string) (QueueSettings, error) {
	cfg := queueSettingsDefault
	if sec, err := rootCfg.GetSection("queue"); err == nil {
		if err = sec.MapTo(&cfg); err != nil {
			log.Error("Failed to map queue common config for %q: %v", name, err)
			return cfg, nil
		}
	}
	if sec, err := rootCfg.GetSection("queue." + name); err == nil {
		if err = sec.MapTo(&cfg); err != nil {
			log.Error("Failed to map queue spec config for %q: %v", name, err)
			return cfg, nil
		}
		if sec.HasKey("CONN_STR") {
			cfg.ConnStr = sec.Key("CONN_STR").String()
		}
	}

	if cfg.Datadir == "" {
		cfg.Datadir = "queues/"
	}
	if !filepath.IsAbs(cfg.Datadir) {
		cfg.Datadir = filepath.Join(AppDataPath, cfg.Datadir)
	}
	cfg.Datadir = filepath.ToSlash(cfg.Datadir)

	if cfg.Type == "redis" && cfg.ConnStr == "" {
		cfg.ConnStr = "redis://127.0.0.1:6379/0"
	}

	if cfg.Length <= 0 {
		cfg.Length = queueSettingsDefault.Length
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = queueSettingsDefault.MaxWorkers
	}
	if cfg.BatchLength <= 0 {
		cfg.BatchLength = queueSettingsDefault.BatchLength
	}

	return cfg, nil
}

// LoadQueueSettings sets up the default settings for Queues
// This is exported for tests to be able to use the queue
func LoadQueueSettings() {
	loadQueueFrom(CfgProvider)
}

func loadQueueFrom(rootCfg ConfigProvider) {
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_TYPE")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "UPDATE_BUFFER_LEN")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "DATADIR")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "CONN_STR")
	handleOldLengthConfiguration(rootCfg, "mailer", "mailer", "SEND_BUFFER_LEN")
	handleOldLengthConfiguration(rootCfg, "pr_patch_checker", "repository", "PULL_REQUEST_QUEUE_LENGTH")
	handleOldLengthConfiguration(rootCfg, "mirror", "repository", "MIRROR_QUEUE_LENGTH")
}

// handleOldLengthConfiguration allows fallback to older configuration. `[queue.name]` `LENGTH` will override this configuration, but
// if that is left unset then we should fall back to the older configuration. (Except where the new length woul be <=0)
func handleOldLengthConfiguration(rootCfg ConfigProvider, newQueueName, oldSection, oldKey string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Fatal("Removed queue option:`[%s].%s`. Use new options in`[queue.%s]`", oldSection, oldKey, newQueueName)
	}
}
