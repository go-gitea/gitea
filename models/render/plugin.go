// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package render

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// Plugin represents a frontend render plugin installed on the instance.
type Plugin struct {
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

func init() {
	db.RegisterModel(new(Plugin))
}

// TableName implements xorm's table name convention.
func (Plugin) TableName() string {
	return "render_plugin"
}

// ListPlugins returns all registered render plugins ordered by identifier.
func ListPlugins(ctx context.Context) ([]*Plugin, error) {
	plugins := make([]*Plugin, 0, 4)
	return plugins, db.GetEngine(ctx).Asc("identifier").Find(&plugins)
}

// ListEnabledPlugins returns all enabled render plugins.
func ListEnabledPlugins(ctx context.Context) ([]*Plugin, error) {
	plugins := make([]*Plugin, 0, 4)
	return plugins, db.GetEngine(ctx).
		Where("enabled = ?", true).
		Asc("identifier").
		Find(&plugins)
}

// GetPluginByID returns the plugin with the given primary key.
func GetPluginByID(ctx context.Context, id int64) (*Plugin, error) {
	plug := new(Plugin)
	has, err := db.GetEngine(ctx).ID(id).Get(plug)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{ID: id}
	}
	return plug, nil
}

// GetPluginByIdentifier returns the plugin with the given identifier.
func GetPluginByIdentifier(ctx context.Context, identifier string) (*Plugin, error) {
	plug := new(Plugin)
	has, err := db.GetEngine(ctx).
		Where("identifier = ?", identifier).
		Get(plug)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: identifier}
	}
	return plug, nil
}

// UpsertPlugin inserts or updates the plugin identified by Identifier.
func UpsertPlugin(ctx context.Context, plug *Plugin) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		existing := new(Plugin)
		has, err := db.GetEngine(ctx).
			Where("identifier = ?", plug.Identifier).
			Get(existing)
		if err != nil {
			return err
		}
		if has {
			plug.ID = existing.ID
			plug.Enabled = existing.Enabled
			plug.CreatedUnix = existing.CreatedUnix
			_, err = db.GetEngine(ctx).
				ID(existing.ID).
				AllCols().
				Update(plug)
			return err
		}
		_, err = db.GetEngine(ctx).Insert(plug)
		return err
	})
}

// SetPluginEnabled toggles plugin enabled state.
func SetPluginEnabled(ctx context.Context, plug *Plugin, enabled bool) error {
	if plug.Enabled == enabled {
		return nil
	}
	plug.Enabled = enabled
	_, err := db.GetEngine(ctx).
		ID(plug.ID).
		Cols("enabled").
		Update(plug)
	return err
}

// DeletePlugin removes the plugin row.
func DeletePlugin(ctx context.Context, plug *Plugin) error {
	_, err := db.GetEngine(ctx).
		ID(plug.ID).
		Delete(new(Plugin))
	return err
}
