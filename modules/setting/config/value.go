// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"strconv"
	"sync"
)

type CfgSecKey struct {
	Sec, Key string
}

type base[T any] struct {
	mu sync.RWMutex

	cfgSecKey CfgSecKey
	dynKey    string

	def, value T
	revision   int
}

func (base *base[T]) parse(s string) (v T) {
	switch any(v).(type) {
	case bool:
		b, _ := strconv.ParseBool(s)
		return any(b).(T)
	default:
		panic("unsupported config type, please complete the code")
	}
}

func (base *base[T]) GetValue() (v T) {
	dg := GetDynGetter()
	if dg == nil {
		// this is an edge case: the database is not initialized but the system setting is going to be used
		// it should panic to avoid inconsistent config values (from config / system setting) and fix the code
		panic("no config dyn value getter")
	}

	rev := dg.GetRevision()

	// if the revision in database doesn't change, use the last value
	base.mu.RLock()
	if rev == base.revision {
		v = base.value
		base.mu.RUnlock()
		return v
	}
	base.mu.RUnlock()

	// try to parse the config and cache it
	var valStr *string
	if dynVal, has := dg.GetValue(base.dynKey); has {
		valStr = &dynVal
	} else if cfgVal, has := GetCfgSecKeyGetter().GetValue(base.cfgSecKey.Sec, base.cfgSecKey.Key); has {
		valStr = &cfgVal
	}
	if valStr == nil {
		v = base.def
	} else {
		v = base.parse(*valStr)
	}

	base.mu.Lock()
	base.value = v
	base.revision = rev
	base.mu.Unlock()
	return v
}

type Value[T any] struct {
	base base[T]
}

func (value *Value[T]) Value() T {
	return value.base.GetValue()
}

func (value *Value[T]) DynKey() string {
	return value.base.dynKey
}

func Bool(def bool, cfgSecKey CfgSecKey, dynKey string) *Value[bool] {
	return &Value[bool]{base: base[bool]{def: def, cfgSecKey: cfgSecKey, dynKey: dynKey}}
}
