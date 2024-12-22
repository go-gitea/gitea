// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package reqctx

type ContextData map[string]any

type ContextDataStore interface {
	GetData() ContextData
}

func (ds ContextData) GetData() ContextData {
	return ds
}

func (ds ContextData) MergeFrom(other ContextData) ContextData {
	for k, v := range other {
		ds[k] = v
	}
	return ds
}
