// Copyright 2013 Beego Authors
// Copyright 2014 The Macaron Authors
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

// fileBackend stores each session in its own file, using the file's modification time as the last access time
type fileBackend struct {
	lock        sync.RWMutex // exclusive for removals, so they never interleave with a load or save
	rootPath    string
	maxLifetime time.Duration
}

func newFileBackend(rootPath string, maxLifetime int64) *fileBackend {
	return &fileBackend{rootPath: filepath.Clean(rootPath), maxLifetime: time.Duration(maxLifetime) * time.Second}
}

func (b *fileBackend) filepath(sid string) string {
	return filepath.Join(b.rootPath, sid[0:1], sid[1:2], sid)
}

func ignoreNotExist(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (b *fileBackend) load(sid string) ([]byte, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	filename := b.filepath(sid)
	stat, err := os.Lstat(filename)
	if err != nil {
		return nil, ignoreNotExist(err)
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("session file %s is not a regular file", filename)
	}
	if time.Since(stat.ModTime()) > b.maxLifetime {
		return nil, nil
	}
	data, err := os.ReadFile(filename)
	if err == nil {
		now := time.Now()
		err = os.Chtimes(filename, now, now)
	}
	return data, ignoreNotExist(err)
}

func (b *fileBackend) save(sid string, data []byte, create bool) error {
	b.lock.RLock()
	defer b.lock.RUnlock()
	filename := b.filepath(sid)
	if create {
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			return err
		}
	} else if _, err := os.Lstat(filename); err != nil {
		return ignoreNotExist(err)
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(filename), sid+".*.tmp")
	if err != nil {
		return err
	}
	_, err = tmpFile.Write(data)
	if err = errors.Join(err, tmpFile.Close()); err == nil {
		err = util.RenameWithRetry(tmpFile.Name(), filename)
	}
	if err != nil {
		_ = os.Remove(tmpFile.Name())
	}
	return err
}

func (b *fileBackend) destroy(sid string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	return ignoreNotExist(os.Remove(b.filepath(sid)))
}

func (b *fileBackend) expired(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && time.Since(info.ModTime()) > b.maxLifetime
}

func (b *fileBackend) gc() {
	err := filepath.WalkDir(b.rootPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !b.expired(path) {
			return ignoreNotExist(err)
		}
		b.lock.Lock()
		defer b.lock.Unlock()
		if b.expired(path) { // a concurrent load may have refreshed it
			err = os.Remove(path)
		}
		return ignoreNotExist(err)
	})
	if err != nil {
		log.Error("Unable to garbage collect session files: %v", err)
	}
}
