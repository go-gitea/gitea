// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "sync"

type GenericSyncMap[K comparable, V any] struct {
	m sync.Map
}

func (m *GenericSyncMap[K, V]) cast(v any, b bool) (ret V, _ bool) {
	if v == nil {
		return ret, b // use zero value for nil item in the map
	}
	return v.(V), b //nolint:forcetypeassert // must be type V
}

func (m *GenericSyncMap[K, V]) Load(key K) (value V, ok bool) {
	return m.cast(m.m.Load(key))
}

func (m *GenericSyncMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

func (m *GenericSyncMap[K, V]) Clear() {
	m.m.Clear()
}

func (m *GenericSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	return m.cast(m.m.LoadOrStore(key, value))
}

func (m *GenericSyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	return m.cast(m.m.LoadAndDelete(key))
}

func (m *GenericSyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

func (m *GenericSyncMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	return m.cast(m.m.Swap(key, value))
}

func (m *GenericSyncMap[K, V]) CompareAndSwap(key K, oldV, newV V) (swapped bool) {
	return m.m.CompareAndSwap(key, oldV, newV)
}

func (m *GenericSyncMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

func (m *GenericSyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V)) //nolint:forcetypeassert // must be type K and V
	})
}
