// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package objectpool

// ObjectPool defines the interface for a git object pool that can query object info and content.
type ObjectPool interface {
	// QueryInfo queries the object info from the git repository by its object name using "git cat-file --batch" family commands.
	// "git cat-file" accepts "<rev>" for the object name, it can be a ref name, object id, etc. https://git-scm.com/docs/gitrevisions
	// In Gitea, we only use the simple ref name or object id, no other complex rev syntax like "suffix" or "git describe" although they are supported by git.
	QueryInfo(obj string) (*Object, error)

	// QueryContent is similar to QueryInfo, it queries the object info and additionally returns a reader for its content.
	// FIXME: this design still follows the old pattern: the returned BufferedReader is very fragile,
	// callers should carefully maintain its lifecycle and discard all unread data.
	// TODO: It needs to be refactored to a fully managed Reader stream in the future, don't let callers manually Close or Discard
	QueryContent(obj string) (*Object, BufferedReader, error)
}
