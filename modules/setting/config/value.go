// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"reflect"
	"sync"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type CfgSecKey struct {
	Sec, Key string
}

// OptionInterface is used to overcome Golang's generic interface limitation
type OptionInterface interface {
	GetDefaultValue() any
}

type Option[T any] struct {
	mu sync.RWMutex

	cfgSecKey CfgSecKey
	dynKey    string

	value      T
	defSimple  T
	defFunc    func() T
	emptyAsDef bool
	has        bool
	revision   int
}

func (opt *Option[T]) GetDefaultValue() any {
	return opt.DefaultValue()
}

func (opt *Option[T]) parse(key, valStr string) (v T) {
	v = opt.DefaultValue()
	if valStr != "" {
		if err := json.Unmarshal(util.UnsafeStringToBytes(valStr), &v); err != nil {
			log.Error("Unable to unmarshal json config for key %q, err: %v", key, err)
		}
	}
	return v
}

func (opt *Option[T]) HasValue(ctx context.Context) bool {
	_, _, has := opt.ValueRevision(ctx)
	return has
}

func (opt *Option[T]) Value(ctx context.Context) (v T) {
	v, _, _ = opt.ValueRevision(ctx)
	return v
}

func isZeroOrEmpty(v any) bool {
	if v == nil {
		return true // interface itself is nil
	}
	r := reflect.ValueOf(v)
	if r.IsZero() {
		return true
	}

	if r.Kind() == reflect.Slice || r.Kind() == reflect.Map {
		if r.IsNil() {
			return true
		}
		return r.Len() == 0
	}
	return false
}

func (opt *Option[T]) ValueRevision(ctx context.Context) (v T, rev int, has bool) {
	dg := GetDynGetter()
	if dg == nil {
		// this is an edge case: the database is not initialized but the system setting is going to be used
		// it should panic to avoid inconsistent config values (from config / system setting) and fix the code
		panic("no config dyn value getter")
	}

	rev = dg.GetRevision(ctx)

	// if the revision in the database doesn't change, use the last value
	opt.mu.RLock()
	if rev == opt.revision {
		v = opt.value
		has = opt.has
		opt.mu.RUnlock()
		return v, rev, has
	}
	opt.mu.RUnlock()

	// try to parse the config and cache it
	var valStr *string
	if dynVal, hasDbValue := dg.GetValue(ctx, opt.dynKey); hasDbValue {
		valStr = &dynVal
	} else if cfgVal, has := GetCfgSecKeyGetter().GetValue(opt.cfgSecKey.Sec, opt.cfgSecKey.Key); has {
		valStr = &cfgVal
	}
	if valStr == nil {
		v = opt.DefaultValue()
		has = false
	} else {
		v = opt.parse(opt.dynKey, *valStr)
		if opt.emptyAsDef && isZeroOrEmpty(v) {
			v = opt.DefaultValue()
		} else {
			has = true
		}
	}

	opt.mu.Lock()
	opt.value = v
	opt.revision = rev
	opt.has = has
	opt.mu.Unlock()
	return v, rev, has
}

func (opt *Option[T]) DynKey() string {
	return opt.dynKey
}

// WithDefaultFunc sets the default value with a function
// The "def" value might be changed during runtime (e.g.: Unmarshal with default), so it shouldn't use the same pointer or slice
func (opt *Option[T]) WithDefaultFunc(f func() T) *Option[T] {
	opt.defFunc = f
	return opt
}

func (opt *Option[T]) WithDefaultSimple(def T) *Option[T] {
	v := any(def)
	switch v.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
	default:
		// TODO: use reflect to support convertable basic types like `type State string`
		r := reflect.ValueOf(v)
		if r.Kind() != reflect.Struct {
			panic("invalid type for default value, use WithDefaultFunc instead")
		}
	}
	opt.defSimple = def
	return opt
}

func (opt *Option[T]) WithEmptyAsDefault() *Option[T] {
	opt.emptyAsDef = true
	return opt
}

func (opt *Option[T]) DefaultValue() T {
	if opt.defFunc != nil {
		return opt.defFunc()
	}
	return opt.defSimple
}

func (opt *Option[T]) WithFileConfig(cfgSecKey CfgSecKey) *Option[T] {
	opt.cfgSecKey = cfgSecKey
	return opt
}

var allConfigOptions = map[string]OptionInterface{}

func NewOption[T any](dynKey string) *Option[T] {
	v := &Option[T]{dynKey: dynKey}
	allConfigOptions[dynKey] = v
	return v
}

func GetConfigOption(dynKey string) OptionInterface {
	return allConfigOptions[dynKey]
}
