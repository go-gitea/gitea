// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// queue package implements a specialized queue system for Gitea.
//

package queue

import "code.gitea.io/gitea/modules/util"

type HandlerFuncT[T any] func(...T) (unhandled []T)

var ErrAlreadyInQueue = util.NewAlreadyExistErrorf("already in queue")
