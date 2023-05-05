// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
)

type BaseConfig struct {
	ManagedName string

	DataFullDir string
	ConnStr     string
	Length      int

	QueueFullName, SetFullName string
}

func toBaseConfig(managedName string, queueSetting setting.QueueSettings) *BaseConfig {
	baseConfig := &BaseConfig{
		ManagedName: managedName,

		ConnStr: queueSetting.ConnStr,
		Length:  queueSetting.Length,
	}
	// data dir
	baseConfig.DataFullDir = queueSetting.Datadir
	if baseConfig.DataFullDir == "" {
		baseConfig.DataFullDir = "queues/"
	}
	baseConfig.DataFullDir = filepath.Clean(baseConfig.DataFullDir)
	if !filepath.IsAbs(baseConfig.DataFullDir) {
		baseConfig.DataFullDir = filepath.Join(setting.AppDataPath, baseConfig.DataFullDir)
	}

	// queue name and set name
	baseConfig.QueueFullName = managedName + queueSetting.QueueName
	baseConfig.SetFullName = baseConfig.QueueFullName + queueSetting.SetName
	if baseConfig.SetFullName == baseConfig.QueueFullName {
		baseConfig.SetFullName += "_unique"
	}
	return baseConfig
}
