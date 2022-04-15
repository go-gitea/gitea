// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	// Implementations related to push mirrors are in `services/pushmirror` package
	// to avoid circular imports
	"code.gitea.io/gitea/services/pushmirror"
)

// AddPushMirrorRemote registers the push mirror remote.
var AddPushMirrorRemote = pushmirror.AddPushMirrorRemote

// RemovePushMirrorRemote removes the push mirror remote.
var RemovePushMirrorRemote = pushmirror.RemovePushMirrorRemote

// SyncPushMirror starts the sync of the push mirror and schedules the next run.
var SyncPushMirror = pushmirror.SyncPushMirror
