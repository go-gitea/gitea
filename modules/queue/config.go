// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import (
	"code.gitea.io/gitea/modules/setting"
)

type BaseConfig struct {
	ManagedName string
	DataFullDir string // the caller must prepare an absolute path

	ConnStr string
	Length  int

	QueueFullName, SetFullName string
}

func toBaseConfig(managedName string, queueSetting setting.QueueSettings) *BaseConfig {
	baseConfig := &BaseConfig{
		ManagedName: managedName,
		DataFullDir: queueSetting.Datadir,

		ConnStr: queueSetting.ConnStr,
		Length:  queueSetting.Length,
	}

	// queue name and set name
	baseConfig.QueueFullName = managedName + queueSetting.QueueName
	baseConfig.SetFullName = baseConfig.QueueFullName + queueSetting.SetName
	if baseConfig.SetFullName == baseConfig.QueueFullName {
		baseConfig.SetFullName += "_unique"
	}
	return baseConfig
}
