// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

// DataStore represents a data store
type DataStore interface {
	GetData() map[string]interface{}
}
