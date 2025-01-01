// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"sync"
)

var getterMu sync.RWMutex

type CfgSecKeyGetter interface {
	GetValue(sec, key string) (v string, has bool)
}

var cfgSecKeyGetterInternal CfgSecKeyGetter

func SetCfgSecKeyGetter(p CfgSecKeyGetter) {
	getterMu.Lock()
	cfgSecKeyGetterInternal = p
	getterMu.Unlock()
}

func GetCfgSecKeyGetter() CfgSecKeyGetter {
	getterMu.RLock()
	defer getterMu.RUnlock()
	return cfgSecKeyGetterInternal
}

type DynKeyGetter interface {
	GetValue(ctx context.Context, key string) (v string, has bool)
	GetRevision(ctx context.Context) int
	InvalidateCache()
}

var dynKeyGetterInternal DynKeyGetter

func SetDynGetter(p DynKeyGetter) {
	getterMu.Lock()
	dynKeyGetterInternal = p
	getterMu.Unlock()
}

func GetDynGetter() DynKeyGetter {
	getterMu.RLock()
	defer getterMu.RUnlock()
	return dynKeyGetterInternal
}
