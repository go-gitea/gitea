// Copyright 2015 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmagic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"
)

// FileStorage facilitates forming file paths derived from a root
// directory. It is used to get file paths in a consistent,
// cross-platform way or persisting ACME assets on the file system.
type FileStorage struct {
	Path string
}

// Exists returns true if key exists in fs.
func (fs *FileStorage) Exists(key string) bool {
	_, err := os.Stat(fs.Filename(key))
	return !os.IsNotExist(err)
}

// Store saves value at key.
func (fs *FileStorage) Store(key string, value []byte) error {
	filename := fs.Filename(key)
	err := os.MkdirAll(filepath.Dir(filename), 0700)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, value, 0600)
}

// Load retrieves the value at key.
func (fs *FileStorage) Load(key string) ([]byte, error) {
	contents, err := ioutil.ReadFile(fs.Filename(key))
	if os.IsNotExist(err) {
		return nil, ErrNotExist(err)
	}
	return contents, nil
}

// Delete deletes the value at key.
func (fs *FileStorage) Delete(key string) error {
	err := os.Remove(fs.Filename(key))
	if os.IsNotExist(err) {
		return ErrNotExist(err)
	}
	return err
}

// List returns all keys that match prefix.
func (fs *FileStorage) List(prefix string, recursive bool) ([]string, error) {
	var keys []string
	walkPrefix := fs.Filename(prefix)

	err := filepath.Walk(walkPrefix, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return fmt.Errorf("%s: file info is nil", fpath)
		}
		if fpath == walkPrefix {
			return nil
		}

		suffix, err := filepath.Rel(walkPrefix, fpath)
		if err != nil {
			return fmt.Errorf("%s: could not make path relative: %v", fpath, err)
		}
		keys = append(keys, path.Join(prefix, suffix))

		if !recursive && info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})

	return keys, err
}

// Stat returns information about key.
func (fs *FileStorage) Stat(key string) (KeyInfo, error) {
	fi, err := os.Stat(fs.Filename(key))
	if os.IsNotExist(err) {
		return KeyInfo{}, ErrNotExist(err)
	}
	if err != nil {
		return KeyInfo{}, err
	}
	return KeyInfo{
		Key:        key,
		Modified:   fi.ModTime(),
		Size:       fi.Size(),
		IsTerminal: !fi.IsDir(),
	}, nil
}

// Filename returns the key as a path on the file
// system prefixed by fs.Path.
func (fs *FileStorage) Filename(key string) string {
	return filepath.Join(fs.Path, filepath.FromSlash(key))
}

// Lock obtains a lock named by the given key. It blocks
// until the lock can be obtained or an error is returned.
func (fs *FileStorage) Lock(ctx context.Context, key string) error {
	filename := fs.lockFilename(key)

	for {
		err := createLockfile(filename)
		if err == nil {
			// got the lock, yay
			return nil
		}
		if !os.IsExist(err) {
			// unexpected error
			return fmt.Errorf("creating lock file: %v", err)
		}

		// lock file already exists

		var meta lockMeta
		f, err := os.Open(filename)
		if err == nil {
			err2 := json.NewDecoder(f).Decode(&meta)
			f.Close()
			if err2 != nil {
				return fmt.Errorf("decoding lockfile contents: %w", err2)
			}
		}

		switch {
		case os.IsNotExist(err):
			// must have just been removed; try again to create it
			continue

		case err != nil:
			// unexpected error
			return fmt.Errorf("accessing lock file: %v", err)

		case fileLockIsStale(meta):
			// lock file is stale - delete it and try again to create one
			log.Printf("[INFO][%s] Lock for '%s' is stale (created: %s, last update: %s); removing then retrying: %s",
				fs, key, meta.Created, meta.Updated, filename)
			removeLockfile(filename)
			continue

		default:
			// lockfile exists and is not stale;
			// just wait a moment and try again,
			// or return if context cancelled
			select {
			case <-time.After(fileLockPollInterval):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// Unlock releases the lock for name.
func (fs *FileStorage) Unlock(key string) error {
	return removeLockfile(fs.lockFilename(key))
}

func (fs *FileStorage) String() string {
	return "FileStorage:" + fs.Path
}

func (fs *FileStorage) lockFilename(key string) string {
	return filepath.Join(fs.lockDir(), StorageKeys.Safe(key)+".lock")
}

func (fs *FileStorage) lockDir() string {
	return filepath.Join(fs.Path, "locks")
}

func fileLockIsStale(meta lockMeta) bool {
	ref := meta.Updated
	if ref.IsZero() {
		ref = meta.Created
	}
	// since updates are exactly every lockFreshnessInterval,
	// add a grace period for the actual file read+write to
	// take place
	return time.Since(ref) > lockFreshnessInterval*2
}

// createLockfile atomically creates the lockfile
// identified by filename. A successfully created
// lockfile should be removed with removeLockfile.
func createLockfile(filename string) error {
	err := atomicallyCreateFile(filename, true)
	if err != nil {
		return err
	}

	go keepLockfileFresh(filename)

	// if the app crashes in removeLockfile(), there is a
	// small chance the .unlock file is left behind; it's
	// safe to simply remove it as it's a guard against
	// double removal of the .lock file.
	_ = os.Remove(filename + ".unlock")
	return nil
}

// removeLockfile atomically removes filename,
// which must be a lockfile created by createLockfile.
// See discussion in PR #7 for more background:
// https://github.com/caddyserver/certmagic/pull/7
func removeLockfile(filename string) error {
	unlockFilename := filename + ".unlock"
	if err := atomicallyCreateFile(unlockFilename, false); err != nil {
		if os.IsExist(err) {
			// another process is handling the unlocking
			return nil
		}
		return err
	}
	defer os.Remove(unlockFilename)
	return os.Remove(filename)
}

// keepLockfileFresh continuously updates the lock file
// at filename with the current timestamp. It stops
// when the file disappears (happy path = lock released),
// or when there is an error at any point. Since it polls
// every lockFreshnessInterval, this function might
// not terminate until up to lockFreshnessInterval after
// the lock is released.
func keepLockfileFresh(filename string) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic: active locking: %v\n%s", err, buf)
		}
	}()

	for {
		time.Sleep(lockFreshnessInterval)
		done, err := updateLockfileFreshness(filename)
		if err != nil {
			log.Printf("[ERROR] Keeping lock file fresh: %v - terminating lock maintenance (lockfile: %s)", err, filename)
			return
		}
		if done {
			return
		}
	}
}

// updateLockfileFreshness updates the lock file at filename
// with the current timestamp. It returns true if the parent
// loop can terminate (i.e. no more need to update the lock).
func updateLockfileFreshness(filename string) (bool, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if os.IsNotExist(err) {
		return true, nil // lock released
	}
	if err != nil {
		return true, err
	}
	defer f.Close()

	// read contents
	metaBytes, err := ioutil.ReadAll(io.LimitReader(f, 2048))
	if err != nil {
		return true, err
	}
	var meta lockMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return true, err
	}

	// truncate file and reset I/O offset to beginning
	if err := f.Truncate(0); err != nil {
		return true, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return true, err
	}

	// write updated timestamp
	meta.Updated = time.Now()
	if err = json.NewEncoder(f).Encode(meta); err != nil {
		return false, err
	}

	// sync to device; we suspect that sometimes file systems
	// (particularly AWS EFS) don't do this on their own,
	// leaving the file empty when we close it; see
	// https://github.com/caddyserver/caddy/issues/3954
	return false, f.Sync()
}

// atomicallyCreateFile atomically creates the file
// identified by filename if it doesn't already exist.
func atomicallyCreateFile(filename string, writeLockInfo bool) error {
	// no need to check this error, we only really care about the file creation error
	_ = os.MkdirAll(filepath.Dir(filename), 0700)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if writeLockInfo {
		now := time.Now()
		meta := lockMeta{
			Created: now,
			Updated: now,
		}
		if err := json.NewEncoder(f).Encode(meta); err != nil {
			return err
		}
		// see https://github.com/caddyserver/caddy/issues/3954
		if err := f.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// homeDir returns the best guess of the current user's home
// directory from environment variables. If unknown, "." (the
// current directory) is returned instead.
func homeDir() string {
	home := os.Getenv("HOME")
	if home == "" && runtime.GOOS == "windows" {
		drive := os.Getenv("HOMEDRIVE")
		path := os.Getenv("HOMEPATH")
		home = drive + path
		if drive == "" || path == "" {
			home = os.Getenv("USERPROFILE")
		}
	}
	if home == "" {
		home = "."
	}
	return home
}

func dataDir() string {
	baseDir := filepath.Join(homeDir(), ".local", "share")
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		baseDir = xdgData
	}
	return filepath.Join(baseDir, "certmagic")
}

// lockMeta is written into a lock file.
type lockMeta struct {
	Created time.Time `json:"created,omitempty"`
	Updated time.Time `json:"updated,omitempty"`
}

// lockFreshnessInterval is how often to update
// a lock's timestamp. Locks with a timestamp
// more than this duration in the past (plus a
// grace period for latency) can be considered
// stale.
const lockFreshnessInterval = 5 * time.Second

// fileLockPollInterval is how frequently
// to check the existence of a lock file
const fileLockPollInterval = 1 * time.Second

// Interface guard
var _ Storage = (*FileStorage)(nil)
