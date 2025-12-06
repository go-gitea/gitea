// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// AddRenderPluginTable creates the render_plugin table used by the frontend plugin system.
func AddRenderPluginTable(x *xorm.Engine) error {
	type RenderPlugin struct {
		ID            int64              `xorm:"pk autoincr"`
		Identifier    string             `xorm:"UNIQUE NOT NULL"`
		Name          string             `xorm:"NOT NULL"`
		Version       string             `xorm:"NOT NULL"`
		Description   string             `xorm:"TEXT"`
		Source        string             `xorm:"TEXT"`
		Entry         string             `xorm:"NOT NULL"`
		FilePatterns  []string           `xorm:"JSON"`
		FormatVersion int                `xorm:"NOT NULL DEFAULT 1"`
		Enabled       bool               `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated NOT NULL"`
	}

	return x.Sync(new(RenderPlugin))
}
