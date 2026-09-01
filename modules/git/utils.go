// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"sync"

	"gitea.dev/modules/util"
)

// ObjectCache provides thread-safe cache operations.
type ObjectCache[T any] struct {
	lock  sync.RWMutex
	cache map[string]T
}

func newObjectCache[T any]() *ObjectCache[T] {
	return &ObjectCache[T]{cache: make(map[string]T, 10)}
}

// Set adds obj to cache
func (oc *ObjectCache[T]) Set(id string, obj T) {
	oc.lock.Lock()
	defer oc.lock.Unlock()

	oc.cache[id] = obj
}

// Get gets cached obj by id
func (oc *ObjectCache[T]) Get(id string) (T, bool) {
	oc.lock.RLock()
	defer oc.lock.RUnlock()

	obj, has := oc.cache[id]
	return obj, has
}

func HashFilePathForWebUI(s string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func SplitCommitTitleBody(commitMessage string, titleRuneLimit int) (title, body string) {
	title, body, _ = strings.Cut(commitMessage, "\n")
	title, title2 := util.EllipsisTruncateRunes(title, titleRuneLimit)
	if title2 != "" {
		if body == "" {
			body = title2
		} else {
			body = title2 + "\n" + body
		}
	}
	return title, body
}
