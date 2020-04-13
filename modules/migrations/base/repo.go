// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// Repository defines a standard repository information
type Repository struct {
	Name         string
	Owner        string
	IsPrivate    bool
	IsMirror     bool
	Description  string
	AuthUsername string
	AuthPassword string
	CloneURL     string
	OriginalURL  string
}
