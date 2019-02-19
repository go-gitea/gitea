// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

// Queue defines an interface to save an issue indexer queue
type Queue interface {
	Run() error
	Push(*IndexerData)
}
