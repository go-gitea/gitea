// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

// DataStore represents a data store
type DataStore interface {
	GetData() map[string]interface{}
}
