// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
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

// ConcatenateError concatenats an error with stderr string
func ConcatenateError(err error, stderr string) error {
	if len(stderr) == 0 {
		return err
	}
	return fmt.Errorf("%w - %s", err, stderr)
}

// ParseBool returns the boolean value represented by the string as per git's git_config_bool
// true will be returned for the result if the string is empty, but valid will be false.
// "true", "yes", "on" are all true, true
// "false", "no", "off" are all false, true
// 0 is false, true
// Any other integer is true, true
// Anything else will return false, false
func ParseBool(value string) (result, valid bool) {
	// Empty strings are true but invalid
	if len(value) == 0 {
		return true, false
	}
	// These are the git expected true and false values
	if strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on") {
		return true, true
	}
	if strings.EqualFold(value, "false") || strings.EqualFold(value, "no") || strings.EqualFold(value, "off") {
		return false, true
	}
	// Try a number
	intValue, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return false, false
	}
	return intValue != 0, true
}

// LimitedReaderCloser is a limited reader closer
type LimitedReaderCloser struct {
	R io.Reader
	C io.Closer
	N int64
}

// Read implements io.Reader
func (l *LimitedReaderCloser) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		_ = l.C.Close()
		return 0, io.EOF
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return n, err
}

// Close implements io.Closer
func (l *LimitedReaderCloser) Close() error {
	return l.C.Close()
}

func HashFilePathForWebUI(s string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
