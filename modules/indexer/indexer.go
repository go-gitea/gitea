// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"code.gitea.io/gitea/models"
)

// NewContext start indexer service
func NewContext() {
	models.InitIssueIndexer()
}
