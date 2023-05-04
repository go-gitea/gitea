// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"path/filepath"
	"unicode"

	"code.gitea.io/gitea/modules/setting"
)

type BaseConfig struct {
	ManagedName string

	DataFullDir string
	ConnStr     string
	Length      int

	QueueFullName, SetFullName string
}

type IniConfig struct {
	Type    string
	Datadir string
	ConnStr string
	Length  int

	QueueName, SetName string

	BatchLength int
	Workers     int
}

var iniConfigDefault = IniConfig{
	Type:    "level",   // dummy, channel, level(leveldb,levelqueue,persistable-channel), redis
	Datadir: "queues/", // relative to AppDataPath
	Length:  1000,      // queue length before a channel queue will block

	QueueName:   "_queue",  // the suffix of the default redis/disk queue name
	SetName:     "_unique", // the suffix of the default redis/disk unique queue set name (for unique queue)
	BatchLength: 20,
	Workers:     1,
}

func joinQueueFullName(managedName, suffixOrName, suffixDefault string) string {
	if suffixOrName == "" {
		suffixOrName = suffixDefault
	}
	if unicode.IsLetter(rune(suffixOrName[0])) {
		return suffixOrName
	}
	return managedName + suffixOrName
}

func toBaseConfig(managedName string, iniCfg *IniConfig) *BaseConfig {
	baseConfig := &BaseConfig{
		ManagedName: managedName,

		ConnStr: iniCfg.ConnStr,
		Length:  iniCfg.Length,
	}
	// data dir
	baseConfig.DataFullDir = iniCfg.Datadir
	if baseConfig.DataFullDir == "" {
		baseConfig.DataFullDir = "queues/"
	}
	baseConfig.DataFullDir = filepath.Clean(baseConfig.DataFullDir)
	if !filepath.IsAbs(baseConfig.DataFullDir) {
		baseConfig.DataFullDir = filepath.Join(setting.AppDataPath, baseConfig.DataFullDir)
	}

	// queue name and set name
	baseConfig.QueueFullName = joinQueueFullName(managedName, iniCfg.QueueName, "_queue")
	baseConfig.SetFullName = joinQueueFullName(managedName, iniCfg.SetName, "_unique")
	return baseConfig
}
