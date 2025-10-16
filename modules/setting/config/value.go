// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"fmt"
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

	cfgSecKey             CfgSecKey
	dynKey, selectFromKey string

	def, value  T
	revision    int
	flipBoolean bool
}

func (value *Value[T]) parse(key, valStr string) (v T) {
	v = value.def
	if valStr != "" {
		if err := json.Unmarshal(util.UnsafeStringToBytes(valStr), &v); err != nil {
			log.Error("Unable to unmarshal json config for key %q, err: %v", key, err)
		}
	}

	return value.invert(v)
}

func (value *Value[T]) invertBoolStr(val string) (inverted string) {
	if val == "true" {
		return "false"
	}

	return "true"
}

func (value *Value[T]) invert(val T) (v T) {
	v = val
	if value.flipBoolean {
		fmt.Printf("Flipping boolean value '%v'...\n", val)
		// if value is of type bool
		if _, ok := any(val).(bool); ok {
			// invert the boolean value upon retrieval
			v = any(!any(val).(bool)).(T)
		} else {
			log.Warn("Ignoring attempt to invert key '%q' for non boolean type", value.selectFromKey)
		}
	}

	return v
}

func (value *Value[T]) getKey() string {
	if value.selectFromKey != "" {
		return value.selectFromKey
	}

	return value.dynKey
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
	if dynVal, has := dg.GetValue(ctx, value.getKey()); has {
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

func (value *Value[T]) SelectFromKey() string {
	return value.selectFromKey
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

func (value *Value[bool]) Invert() *Value[bool] {
	value.flipBoolean = true
	return value
}

func (value *Value[any]) SelectFrom(sectionName string) *Value[any] {
	value.selectFromKey = sectionName
	return value
}

func (value *Value[any]) SetValue(val string) error {
	ctx := context.Background()
	ds := GetDynSetter()
	if ds == nil {
		// this is an edge case: the database is not initialized but the system setting is going to be used
		// it should panic to avoid inconsistent config values (from config / system setting) and fix the code
		panic("no config dyn value getter")
	}

	fmt.Printf("Setting value '%s' with old key '%s' using key '%s'\n", val, value.selectFromKey, value.dynKey)

	if value.flipBoolean {
		return ds.SetValue(ctx, value.getKey(), value.invertBoolStr(val))
	}

	return ds.SetValue(ctx, value.getKey(), val)
}

func ValueJSON[T any](dynKey string) *Value[T] {
	return &Value[T]{dynKey: dynKey}
}
