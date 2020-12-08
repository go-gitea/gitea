// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// ObjectCache represents a cache opeations.
type ObjectCache interface {
	Set(id string, obj interface{})
	Get(id string) (interface{}, bool)
}
