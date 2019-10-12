// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package task

import "code.gitea.io/gitea/models"

// Queue defines an interface to run task queue
type Queue interface {
	Run() error
	Push(*models.Task) error
	Stop()
}
