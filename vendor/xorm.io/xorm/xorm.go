// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.11

package xorm

import (
	"context"
	"os"
	"runtime"
	"time"

	"xorm.io/xorm/caches"
	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/log"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
	"xorm.io/xorm/tags"
)

func close(engine *Engine) {
	engine.Close()
}

// NewEngine new a db manager according to the parameter. Currently support four
// drivers
func NewEngine(driverName string, dataSourceName string) (*Engine, error) {
	dialect, err := dialects.OpenDialect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	db, err := core.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	cacherMgr := caches.NewManager()
	mapper := names.NewCacheMapper(new(names.SnakeMapper))
	tagParser := tags.NewParser("xorm", dialect, mapper, mapper, cacherMgr)

	engine := &Engine{
		dialect:        dialect,
		TZLocation:     time.Local,
		defaultContext: context.Background(),
		cacherMgr:      cacherMgr,
		tagParser:      tagParser,
		driverName:     driverName,
		dataSourceName: dataSourceName,
		db:             db,
	}

	if dialect.URI().DBType == schemas.SQLITE {
		engine.DatabaseTZ = time.UTC
	} else {
		engine.DatabaseTZ = time.Local
	}

	logger := log.NewSimpleLogger(os.Stdout)
	logger.SetLevel(log.LOG_INFO)
	engine.SetLogger(log.NewLoggerAdapter(logger))

	runtime.SetFinalizer(engine, close)

	return engine, nil
}

// NewEngineWithParams new a db manager with params. The params will be passed to dialects.
func NewEngineWithParams(driverName string, dataSourceName string, params map[string]string) (*Engine, error) {
	engine, err := NewEngine(driverName, dataSourceName)
	engine.dialect.SetParams(params)
	return engine, err
}

// Clone clone an engine
func (engine *Engine) Clone() (*Engine, error) {
	return NewEngine(engine.DriverName(), engine.DataSourceName())
}
