// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ObjectCache provides thread-safe cache opeations.
type ObjectCache struct {
	lock  sync.RWMutex
	cache map[string]interface{}
}

func newObjectCache() *ObjectCache {
	return &ObjectCache{
		cache: make(map[string]interface{}, 10),
	}
}

// Set add obj to cache
func (oc *ObjectCache) Set(id string, obj interface{}) {
	oc.lock.Lock()
	defer oc.lock.Unlock()

	oc.cache[id] = obj
}

// Get get cached obj by id
func (oc *ObjectCache) Get(id string) (interface{}, bool) {
	oc.lock.RLock()
	defer oc.lock.RUnlock()

	obj, has := oc.cache[id]
	return obj, has
}

// isDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func isDir(dir string) bool {
	f, e := os.Stat(dir)
	if e != nil {
		return false
	}
	return f.IsDir()
}

// isFile returns true if given path is a file,
// or returns false when it's a directory or does not exist.
func isFile(filePath string) bool {
	f, e := os.Stat(filePath)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

// isExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func concatenateError(err error, stderr string) error {
	if len(stderr) == 0 {
		return err
	}
	return fmt.Errorf("%v - %s", err, stderr)
}

// If the object is stored in its own file (i.e not in a pack file),
// this function returns the full path to the object file.
// It does not test if the file exists.
func filepathFromSHA1(rootdir, sha1 string) string {
	return filepath.Join(rootdir, "objects", sha1[:2], sha1[2:])
}

// RefEndName return the end name of a ref name
func RefEndName(refStr string) string {
	if strings.HasPrefix(refStr, BranchPrefix) {
		return refStr[len(BranchPrefix):]
	}

	if strings.HasPrefix(refStr, TagPrefix) {
		return refStr[len(TagPrefix):]
	}

	return refStr
}
