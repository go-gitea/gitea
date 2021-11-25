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
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Storage is a type that implements a key-value store.
// Keys are prefix-based, with forward slash '/' as separators
// and without a leading slash.
//
// Processes running in a cluster will wish to use the
// same Storage value (its implementation and configuration)
// in order to share certificates and other TLS resources
// with the cluster.
//
// The Load, Delete, List, and Stat methods should return
// ErrNotExist if the key does not exist.
//
// Implementations of Storage must be safe for concurrent use.
type Storage interface {
	// Locker provides atomic synchronization
	// operations, making Storage safe to share.
	Locker

	// Store puts value at key.
	Store(key string, value []byte) error

	// Load retrieves the value at key.
	Load(key string) ([]byte, error)

	// Delete deletes key. An error should be
	// returned only if the key still exists
	// when the method returns.
	Delete(key string) error

	// Exists returns true if the key exists
	// and there was no error checking.
	Exists(key string) bool

	// List returns all keys that match prefix.
	// If recursive is true, non-terminal keys
	// will be enumerated (i.e. "directories"
	// should be walked); otherwise, only keys
	// prefixed exactly by prefix will be listed.
	List(prefix string, recursive bool) ([]string, error)

	// Stat returns information about key.
	Stat(key string) (KeyInfo, error)
}

// Locker facilitates synchronization of certificate tasks across
// machines and networks.
type Locker interface {
	// Lock acquires the lock for key, blocking until the lock
	// can be obtained or an error is returned. Note that, even
	// after acquiring a lock, an idempotent operation may have
	// already been performed by another process that acquired
	// the lock before - so always check to make sure idempotent
	// operations still need to be performed after acquiring the
	// lock.
	//
	// The actual implementation of obtaining of a lock must be
	// an atomic operation so that multiple Lock calls at the
	// same time always results in only one caller receiving the
	// lock at any given time.
	//
	// To prevent deadlocks, all implementations (where this concern
	// is relevant) should put a reasonable expiration on the lock in
	// case Unlock is unable to be called due to some sort of network
	// failure or system crash. Additionally, implementations should
	// honor context cancellation as much as possible (in case the
	// caller wishes to give up and free resources before the lock
	// can be obtained).
	Lock(ctx context.Context, key string) error

	// Unlock releases the lock for key. This method must ONLY be
	// called after a successful call to Lock, and only after the
	// critical section is finished, even if it errored or timed
	// out. Unlock cleans up any resources allocated during Lock.
	Unlock(key string) error
}

// KeyInfo holds information about a key in storage.
// Key and IsTerminal are required; Modified and Size
// are optional if the storage implementation is not
// able to get that information. Setting them will
// make certain operations more consistent or
// predictable, but it is not crucial to basic
// functionality.
type KeyInfo struct {
	Key        string
	Modified   time.Time
	Size       int64
	IsTerminal bool // false for keys that only contain other keys (like directories)
}

// storeTx stores all the values or none at all.
func storeTx(s Storage, all []keyValue) error {
	for i, kv := range all {
		err := s.Store(kv.key, kv.value)
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				s.Delete(all[j].key)
			}
			return err
		}
	}
	return nil
}

// keyValue pairs a key and a value.
type keyValue struct {
	key   string
	value []byte
}

// KeyBuilder provides a namespace for methods that
// build keys and key prefixes, for addressing items
// in a Storage implementation.
type KeyBuilder struct{}

// CertsPrefix returns the storage key prefix for
// the given certificate issuer.
func (keys KeyBuilder) CertsPrefix(issuerKey string) string {
	return path.Join(prefixCerts, keys.Safe(issuerKey))
}

// CertsSitePrefix returns a key prefix for items associated with
// the site given by domain using the given issuer key.
func (keys KeyBuilder) CertsSitePrefix(issuerKey, domain string) string {
	return path.Join(keys.CertsPrefix(issuerKey), keys.Safe(domain))
}

// SiteCert returns the path to the certificate file for domain
// that is associated with the issuer with the given issuerKey.
func (keys KeyBuilder) SiteCert(issuerKey, domain string) string {
	safeDomain := keys.Safe(domain)
	return path.Join(keys.CertsSitePrefix(issuerKey, domain), safeDomain+".crt")
}

// SitePrivateKey returns the path to the private key file for domain
// that is associated with the certificate from the given issuer with
// the given issuerKey.
func (keys KeyBuilder) SitePrivateKey(issuerKey, domain string) string {
	safeDomain := keys.Safe(domain)
	return path.Join(keys.CertsSitePrefix(issuerKey, domain), safeDomain+".key")
}

// SiteMeta returns the path to the metadata file for domain that
// is associated with the certificate from the given issuer with
// the given issuerKey.
func (keys KeyBuilder) SiteMeta(issuerKey, domain string) string {
	safeDomain := keys.Safe(domain)
	return path.Join(keys.CertsSitePrefix(issuerKey, domain), safeDomain+".json")
}

// OCSPStaple returns a key for the OCSP staple associated
// with the given certificate. If you have the PEM bundle
// handy, pass that in to save an extra encoding step.
func (keys KeyBuilder) OCSPStaple(cert *Certificate, pemBundle []byte) string {
	var ocspFileName string
	if len(cert.Names) > 0 {
		firstName := keys.Safe(cert.Names[0])
		ocspFileName = firstName + "-"
	}
	ocspFileName += fastHash(pemBundle)
	return path.Join(prefixOCSP, ocspFileName)
}

// Safe standardizes and sanitizes str for use as
// a single component of a storage key. This method
// is idempotent.
func (keys KeyBuilder) Safe(str string) string {
	str = strings.ToLower(str)
	str = strings.TrimSpace(str)

	// replace a few specific characters
	repl := strings.NewReplacer(
		" ", "_",
		"+", "_plus_",
		"*", "wildcard_",
		":", "-",
		"..", "", // prevent directory traversal (regex allows single dots)
	)
	str = repl.Replace(str)

	// finally remove all non-word characters
	return safeKeyRE.ReplaceAllLiteralString(str, "")
}

// CleanUpOwnLocks immediately cleans up all
// current locks obtained by this process. Since
// this does not cancel the operations that
// the locks are synchronizing, this should be
// called only immediately before process exit.
// Errors are only reported if a logger is given.
func CleanUpOwnLocks(logger *zap.Logger) {
	locksMu.Lock()
	defer locksMu.Unlock()
	for lockKey, storage := range locks {
		err := storage.Unlock(lockKey)
		if err == nil {
			delete(locks, lockKey)
		} else if logger != nil {
			logger.Error("unable to clean up lock in storage backend",
				zap.Any("storage", storage),
				zap.String("lock_key", lockKey),
				zap.Error(err),
			)
		}
	}
}

func acquireLock(ctx context.Context, storage Storage, lockKey string) error {
	err := storage.Lock(ctx, lockKey)
	if err == nil {
		locksMu.Lock()
		locks[lockKey] = storage
		locksMu.Unlock()
	}
	return err
}

func releaseLock(storage Storage, lockKey string) error {
	err := storage.Unlock(lockKey)
	if err == nil {
		locksMu.Lock()
		delete(locks, lockKey)
		locksMu.Unlock()
	}
	return err
}

// locks stores a reference to all the current
// locks obtained by this process.
var locks = make(map[string]Storage)
var locksMu sync.Mutex

// StorageKeys provides methods for accessing
// keys and key prefixes for items in a Storage.
// Typically, you will not need to use this
// because accessing storage is abstracted away
// for most cases. Only use this if you need to
// directly access TLS assets in your application.
var StorageKeys KeyBuilder

const (
	prefixCerts = "certificates"
	prefixOCSP  = "ocsp"
)

// safeKeyRE matches any undesirable characters in storage keys.
// Note that this allows dots, so you'll have to strip ".." manually.
var safeKeyRE = regexp.MustCompile(`[^\w@.-]`)

// ErrNotExist is returned by Storage implementations when
// a resource is not found. It is similar to os.IsNotExist
// except this is a type, not a variable.
// TODO: use new Go error wrapping conventions
type ErrNotExist interface {
	error
}

// defaultFileStorage is a convenient, default storage
// implementation using the local file system.
var defaultFileStorage = &FileStorage{Path: dataDir()}
