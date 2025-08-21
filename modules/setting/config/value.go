// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"sync"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type CfgSecKey struct {
	Sec, Key string
}

type Value[T any] struct {
	mu sync.RWMutex

	cfgSecKey CfgSecKey
	dynKey    string

	def, value T
	revision   int
}

func (value *Value[T]) parse(key, valStr string) (v T) {
	v = value.def
	if valStr != "" {
		if err := json.Unmarshal(util.UnsafeStringToBytes(valStr), &v); err != nil {
			log.Error("Unable to unmarshal json config for key %q, err: %v", key, err)
		}
	}
	return v
}

func (value *Value[T]) Value(ctx context.Context) (v T) {
	dg := GetDynGetter()
	if dg == nil {
		// this is an edge case: the database is not initialized but the system setting is going to be used
		// it should panic to avoid inconsistent config values (from config / system setting) and fix the code
		panic("no config dyn value getter")
	}

	rev := dg.GetRevision(ctx)

	// if the revision in the database doesn't change, use the last value
	value.mu.RLock()
	if rev == value.revision {
		v = value.value
		value.mu.RUnlock()
		return v
	}
	value.mu.RUnlock()

	// try to parse the config and cache it
	var valStr *string
	if dynVal, has := dg.GetValue(ctx, value.dynKey); has {
		valStr = &dynVal
	} else if cfgVal, has := GetCfgSecKeyGetter().GetValue(value.cfgSecKey.Sec, value.cfgSecKey.Key); has {
		valStr = &cfgVal
	}
	if valStr == nil {
		v = value.def
	} else {
		v = value.parse(value.dynKey, *valStr)
	}

	value.mu.Lock()
	value.value = v
	value.revision = rev
	value.mu.Unlock()
	return v
}

func (value *Value[T]) DynKey() string {
	return value.dynKey
}

func (value *Value[T]) WithDefault(def T) *Value[T] {
	value.def = def
	return value
}

func (value *Value[T]) DefaultValue() T {
	return value.def
}

func (value *Value[T]) WithFileConfig(cfgSecKey CfgSecKey) *Value[T] {
	value.cfgSecKey = cfgSecKey
	return value
}

func ValueJSON[T any](dynKey string) *Value[T] {
	return &Value[T]{dynKey: dynKey}
}
