// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubModuleFile *SubModuleFile
}
