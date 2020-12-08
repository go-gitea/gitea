// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// Blob represents a git blob
type Blob interface {
	Object
	Name() string
}
