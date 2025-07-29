// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

func GetMapValueOrDefault[T any](m map[string]any, key string, defaultValue T) T {
	if value, ok := m[key]; ok {
		if v, ok := value.(T); ok {
			return v
		}
	}
	return defaultValue
}
