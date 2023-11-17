// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"
	"math"
	"sync"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/timeutil"
)

type Setting struct {
	ID           int64              `xorm:"pk autoincr"`
	SettingKey   string             `xorm:"varchar(255) unique"` // key should be lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"`
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "system_setting"
}

func init() {
	db.RegisterModel(new(Setting))
}

const keyRevision = "revision"

func GetRevision(ctx context.Context) int {
	revision := &Setting{SettingKey: keyRevision}
	if has, err := db.GetByBean(ctx, revision); err != nil {
		return 0
	} else if !has {
		err = db.Insert(ctx, &Setting{SettingKey: keyRevision, Version: 1})
		if err != nil {
			return 0
		}
		return 1
	} else if revision.Version <= 0 || revision.Version >= math.MaxInt-1 {
		_, err = db.Exec(ctx, "UPDATE system_setting SET version=1 WHERE setting_key=?", keyRevision)
		if err != nil {
			return 0
		}
		return 1
	}
	return revision.Version
}

func GetAllSettings(ctx context.Context) (revision int, res map[string]string, err error) {
	_ = GetRevision(ctx) // prepare the "revision" key ahead
	var settings []*Setting
	if err := db.GetEngine(ctx).
		Find(&settings); err != nil {
		return 0, nil, err
	}
	res = make(map[string]string)
	for _, s := range settings {
		if s.SettingKey == keyRevision {
			revision = s.Version
		}
		res[s.SettingKey] = s.SettingValue
	}
	return revision, res, nil
}

func SetSettings(ctx context.Context, settings map[string]string) error {
	_ = GetRevision(ctx) // prepare the "revision" key ahead
	return db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)
		_, err := db.Exec(ctx, "UPDATE system_setting SET version=version+1 WHERE setting_key=?", keyRevision)
		if err != nil {
			return err
		}
		for k, v := range settings {
			res, err := e.Exec("UPDATE system_setting SET setting_value=? WHERE setting_key=?", v, k)
			if err != nil {
				return err
			}
			rows, _ := res.RowsAffected()
			if rows == 0 { // if no existing row, insert a new row
				if _, err = e.Insert(&Setting{SettingKey: k, SettingValue: v}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

type dbConfigCachedGetter struct {
	mu sync.RWMutex

	cacheTime time.Time
	revision  int
	settings  map[string]string
}

var _ config.DynKeyGetter = (*dbConfigCachedGetter)(nil)

func (d *dbConfigCachedGetter) GetValue(ctx context.Context, key string) (v string, has bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, has = d.settings[key]
	return v, has
}

func (d *dbConfigCachedGetter) GetRevision(ctx context.Context) int {
	d.mu.RLock()
	cachedDuration := time.Since(d.cacheTime)
	cachedRevision := d.revision
	d.mu.RUnlock()

	if cachedDuration < time.Second {
		return cachedRevision
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if GetRevision(ctx) != d.revision {
		rev, set, err := GetAllSettings(ctx)
		if err != nil {
			log.Error("Unable to get all settings: %v", err)
		} else {
			d.revision = rev
			d.settings = set
		}
	}
	d.cacheTime = time.Now()
	return d.revision
}

func (d *dbConfigCachedGetter) InvalidateCache() {
	d.mu.Lock()
	d.cacheTime = time.Time{}
	d.mu.Unlock()
}

func NewDatabaseDynKeyGetter() config.DynKeyGetter {
	return &dbConfigCachedGetter{}
}
