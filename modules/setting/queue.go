// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

// QueueSettings represent the settings for a queue from the ini
type QueueSettings struct {
	Name string // not an INI option, it is the name for [queue.the-name] section

	Type    string
	Datadir string
	ConnStr string // for leveldb or redis
	Length  int    // max queue length before blocking

	QueueName, SetName string // the name suffix for storage (db key, redis key), "set" is for unique queue

	BatchLength int
	MaxWorkers  int
}

var queueSettingsDefault = QueueSettings{
	Type:    "level",         // dummy, channel, level, redis
	Datadir: "queues/common", // relative to AppDataPath
	Length:  100,             // queue length before a channel queue will block

	QueueName:   "_queue",
	SetName:     "_unique",
	BatchLength: 20,
	MaxWorkers:  10,
}

func GetQueueSettings(rootCfg ConfigProvider, name string) (QueueSettings, error) {
	// deep copy default settings
	cfg := QueueSettings{}
	if cfgBs, err := json.Marshal(queueSettingsDefault); err != nil {
		return cfg, err
	} else if err = json.Unmarshal(cfgBs, &cfg); err != nil {
		return cfg, err
	}

	cfg.Name = name
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
		cfg.Datadir = queueSettingsDefault.Datadir
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

func LoadQueueSettings() {
	loadQueueFrom(CfgProvider)
}

func loadQueueFrom(rootCfg ConfigProvider) {
	hasOld := false
	handleOldLengthConfiguration := func(rootCfg ConfigProvider, newQueueName, oldSection, oldKey string) {
		if rootCfg.Section(oldSection).HasKey(oldKey) {
			hasOld = true
			log.Error("Removed queue option: `[%s].%s`. Use new options in `[queue.%s]`", oldSection, oldKey, newQueueName)
		}
	}
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_TYPE")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_BATCH_NUMBER")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_DIR")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "ISSUE_INDEXER_QUEUE_CONN_STR")
	handleOldLengthConfiguration(rootCfg, "issue_indexer", "indexer", "UPDATE_BUFFER_LEN")
	handleOldLengthConfiguration(rootCfg, "mailer", "mailer", "SEND_BUFFER_LEN")
	handleOldLengthConfiguration(rootCfg, "pr_patch_checker", "repository", "PULL_REQUEST_QUEUE_LENGTH")
	handleOldLengthConfiguration(rootCfg, "mirror", "repository", "MIRROR_QUEUE_LENGTH")
	if hasOld {
		log.Fatal("Please update your app.ini to remove deprecated config options")
	}
}
