// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

//  _                       ___       _
// /   _  ._ _  ._ _  o _|_  |  ._  _|_  _
// \_ (_) | | | | | | |  |_ _|_ | |  |  (_)
//

// CommitsInfoService represents a service that provides commits info
type CommitsInfoService interface {
	// GetCommitsInfo gets information of all commits that are corresponding to these entries
	GetCommitsInfo(commit Commit, treePath string, entries Entries, cache LastCommitCache) ([]CommitInfo, Commit, error)

	// NewLastCommitCache creates a new last commit cache for repo
	NewLastCommitCache(gitRepo Repository, ttl int64, cache Cache) LastCommitCache
}

// Cache represents a caching interface
type Cache interface {
	// Put puts value into cache with key and expire time.
	Put(key string, val interface{}, timeout int64) error
	// Get gets cached value by given key.
	Get(key string) interface{}
}

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         TreeEntry
	Commit        Commit
	SubModuleFile SubModuleFile
}

// LastCommitCache represents a caching interface
type LastCommitCache interface {
	// Put puts value into cache with key and expire time.
	Put(ref, entryPath, commitID string) error
	// Get gets cached value by given key.
	Get(ref, entryPath string) (interface{}, error)
	// CacheCommit will cache the commit from the repository
	CacheCommit(commit Commit) error
}
