// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"strconv"
	"sync"
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

func (value *Value[T]) parse(s string) (v T) {
	switch any(v).(type) {
	case bool:
		b, _ := strconv.ParseBool(s)
		return any(b).(T)
	default:
		panic("unsupported config type, please complete the code")
	}
}

func (value *Value[T]) Value(ctx context.Context) (v T) {
	dg := GetDynGetter()
	if dg == nil {
		// this is an edge case: the database is not initialized but the system setting is going to be used
		// it should panic to avoid inconsistent config values (from config / system setting) and fix the code
		panic("no config dyn value getter")
	}

	rev := dg.GetRevision(ctx)

	// if the revision in database doesn't change, use the last value
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
		v = value.parse(*valStr)
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

func Bool(def bool, cfgSecKey CfgSecKey, dynKey string) *Value[bool] {
	return &Value[bool]{def: def, cfgSecKey: cfgSecKey, dynKey: dynKey}
}
