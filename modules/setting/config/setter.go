// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"context"
	"sync"
)

var setterMu sync.RWMutex

type DynKeySetter interface {
	SetValue(ctx context.Context, dynKey, value string) error
}

var dynKeySetterInternal DynKeySetter

func SetDynSetter(p DynKeySetter) {
	setterMu.Lock()
	dynKeySetterInternal = p
	setterMu.Unlock()
}

func GetDynSetter() DynKeySetter {
	getterMu.RLock()
	defer getterMu.RUnlock()
	return dynKeySetterInternal
}
