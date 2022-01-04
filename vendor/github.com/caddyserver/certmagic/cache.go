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
	"fmt"
	weakrand "math/rand" // seeded elsewhere
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Cache is a structure that stores certificates in memory.
// A Cache indexes certificates by name for quick access
// during TLS handshakes, and avoids duplicating certificates
// in memory. Generally, there should only be one per process.
// However, that is not a strict requirement; but using more
// than one is a code smell, and may indicate an
// over-engineered design.
//
// An empty cache is INVALID and must not be used. Be sure
// to call NewCache to get a valid value.
//
// These should be very long-lived values and must not be
// copied. Before all references leave scope to be garbage
// collected, ensure you call Stop() to stop maintenance on
// the certificates stored in this cache and release locks.
//
// Caches are not usually manipulated directly; create a
// Config value with a pointer to a Cache, and then use
// the Config to interact with the cache. Caches are
// agnostic of any particular storage or ACME config,
// since each certificate may be managed and stored
// differently.
type Cache struct {
	// User configuration of the cache
	options CacheOptions

	// The cache is keyed by certificate hash
	cache map[string]Certificate

	// cacheIndex is a map of SAN to cache key (cert hash)
	cacheIndex map[string][]string

	// Protects the cache and index maps
	mu sync.RWMutex

	// Close this channel to cancel asset maintenance
	stopChan chan struct{}

	// Used to signal when stopping is completed
	doneChan chan struct{}

	logger *zap.Logger
}

// NewCache returns a new, valid Cache for efficiently
// accessing certificates in memory. It also begins a
// maintenance goroutine to tend to the certificates
// in the cache. Call Stop() when you are done with the
// cache so it can clean up locks and stuff.
//
// Most users of this package will not need to call this
// because a default certificate cache is created for you.
// Only advanced use cases require creating a new cache.
//
// This function panics if opts.GetConfigForCert is not
// set. The reason is that a cache absolutely needs to
// be able to get a Config with which to manage TLS
// assets, and it is not safe to assume that the Default
// config is always the correct one, since you have
// created the cache yourself.
//
// See the godoc for Cache to use it properly. When
// no longer needed, caches should be stopped with
// Stop() to clean up resources even if the process
// is being terminated, so that it can clean up
// any locks for other processes to unblock!
func NewCache(opts CacheOptions) *Cache {
	// assume default options if necessary
	if opts.OCSPCheckInterval <= 0 {
		opts.OCSPCheckInterval = DefaultOCSPCheckInterval
	}
	if opts.RenewCheckInterval <= 0 {
		opts.RenewCheckInterval = DefaultRenewCheckInterval
	}
	if opts.Capacity < 0 {
		opts.Capacity = 0
	}

	// this must be set, because we cannot not
	// safely assume that the Default Config
	// is always the correct one to use
	if opts.GetConfigForCert == nil {
		panic("cache must be initialized with a GetConfigForCert callback")
	}

	c := &Cache{
		options:    opts,
		cache:      make(map[string]Certificate),
		cacheIndex: make(map[string][]string),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		logger:     opts.Logger,
	}

	go c.maintainAssets(0)

	return c
}

// Stop stops the maintenance goroutine for
// certificates in certCache. It blocks until
// stopping is complete. Once a cache is
// stopped, it cannot be reused.
func (certCache *Cache) Stop() {
	close(certCache.stopChan) // signal to stop
	<-certCache.doneChan      // wait for stop to complete
}

// CacheOptions is used to configure certificate caches.
// Once a cache has been created with certain options,
// those settings cannot be changed.
type CacheOptions struct {
	// REQUIRED. A function that returns a configuration
	// used for managing a certificate, or for accessing
	// that certificate's asset storage (e.g. for
	// OCSP staples, etc). The returned Config MUST
	// be associated with the same Cache as the caller.
	//
	// The reason this is a callback function, dynamically
	// returning a Config (instead of attaching a static
	// pointer to a Config on each certificate) is because
	// the config for how to manage a domain's certificate
	// might change from maintenance to maintenance. The
	// cache is so long-lived, we cannot assume that the
	// host's situation will always be the same; e.g. the
	// certificate might switch DNS providers, so the DNS
	// challenge (if used) would need to be adjusted from
	// the last time it was run ~8 weeks ago.
	GetConfigForCert ConfigGetter

	// How often to check certificates for renewal;
	// if unset, DefaultOCSPCheckInterval will be used.
	OCSPCheckInterval time.Duration

	// How often to check certificates for renewal;
	// if unset, DefaultRenewCheckInterval will be used.
	RenewCheckInterval time.Duration

	// Maximum number of certificates to allow in the cache.
	// If reached, certificates will be randomly evicted to
	// make room for new ones. 0 means unlimited.
	Capacity int

	// Set a logger to enable logging
	Logger *zap.Logger
}

// ConfigGetter is a function that returns a prepared,
// valid config that should be used when managing the
// given certificate or its assets.
type ConfigGetter func(Certificate) (*Config, error)

// cacheCertificate calls unsyncedCacheCertificate with a write lock.
//
// This function is safe for concurrent use.
func (certCache *Cache) cacheCertificate(cert Certificate) {
	certCache.mu.Lock()
	certCache.unsyncedCacheCertificate(cert)
	certCache.mu.Unlock()
}

// unsyncedCacheCertificate adds cert to the in-memory cache unless
// it already exists in the cache (according to cert.Hash). It
// updates the name index.
//
// This function is NOT safe for concurrent use. Callers MUST acquire
// a write lock on certCache.mu first.
func (certCache *Cache) unsyncedCacheCertificate(cert Certificate) {
	// no-op if this certificate already exists in the cache
	if _, ok := certCache.cache[cert.hash]; ok {
		if certCache.logger != nil {
			certCache.logger.Debug("certificate already cached",
				zap.Strings("subjects", cert.Names),
				zap.Time("expiration", cert.Leaf.NotAfter),
				zap.Bool("managed", cert.managed),
				zap.String("issuer_key", cert.issuerKey),
				zap.String("hash", cert.hash))
		}
		return
	}

	// if the cache is at capacity, make room for new cert
	cacheSize := len(certCache.cache)
	if certCache.options.Capacity > 0 && cacheSize >= certCache.options.Capacity {
		// Go maps are "nondeterministic" but not actually random,
		// so although we could just chop off the "front" of the
		// map with less code, that is a heavily skewed eviction
		// strategy; generating random numbers is cheap and
		// ensures a much better distribution.
		rnd := weakrand.Intn(cacheSize)
		i := 0
		for _, randomCert := range certCache.cache {
			if i == rnd {
				if certCache.logger != nil {
					certCache.logger.Debug("cache full; evicting random certificate",
						zap.Strings("removing_subjects", randomCert.Names),
						zap.String("removing_hash", randomCert.hash),
						zap.Strings("inserting_subjects", cert.Names),
						zap.String("inserting_hash", cert.hash))
				}
				certCache.removeCertificate(randomCert)
				break
			}
			i++
		}
	}

	// store the certificate
	certCache.cache[cert.hash] = cert

	// update the index so we can access it by name
	for _, name := range cert.Names {
		certCache.cacheIndex[name] = append(certCache.cacheIndex[name], cert.hash)
	}

	if certCache.logger != nil {
		certCache.logger.Debug("added certificate to cache",
			zap.Strings("subjects", cert.Names),
			zap.Time("expiration", cert.Leaf.NotAfter),
			zap.Bool("managed", cert.managed),
			zap.String("issuer_key", cert.issuerKey),
			zap.String("hash", cert.hash),
			zap.Int("cache_size", len(certCache.cache)),
			zap.Int("cache_capacity", certCache.options.Capacity))
	}
}

// removeCertificate removes cert from the cache.
//
// This function is NOT safe for concurrent use; callers
// MUST first acquire a write lock on certCache.mu.
func (certCache *Cache) removeCertificate(cert Certificate) {
	// delete all mentions of this cert from the name index
	for _, name := range cert.Names {
		keyList := certCache.cacheIndex[name]
		for i := 0; i < len(keyList); i++ {
			if keyList[i] == cert.hash {
				keyList = append(keyList[:i], keyList[i+1:]...)
				i--
			}
		}
		if len(keyList) == 0 {
			delete(certCache.cacheIndex, name)
		} else {
			certCache.cacheIndex[name] = keyList
		}
	}

	// delete the actual cert from the cache
	delete(certCache.cache, cert.hash)

	if certCache.logger != nil {
		certCache.logger.Debug("removed certificate from cache",
			zap.Strings("subjects", cert.Names),
			zap.Time("expiration", cert.Leaf.NotAfter),
			zap.Bool("managed", cert.managed),
			zap.String("issuer_key", cert.issuerKey),
			zap.String("hash", cert.hash),
			zap.Int("cache_size", len(certCache.cache)),
			zap.Int("cache_capacity", certCache.options.Capacity))
	}
}

// replaceCertificate atomically replaces oldCert with newCert in
// the cache.
//
// This method is safe for concurrent use.
func (certCache *Cache) replaceCertificate(oldCert, newCert Certificate) {
	certCache.mu.Lock()
	certCache.removeCertificate(oldCert)
	certCache.unsyncedCacheCertificate(newCert)
	certCache.mu.Unlock()
	if certCache.logger != nil {
		certCache.logger.Info("replaced certificate in cache",
			zap.Strings("subjects", newCert.Names),
			zap.Time("new_expiration", newCert.Leaf.NotAfter))
	}
}

func (certCache *Cache) getAllMatchingCerts(name string) []Certificate {
	certCache.mu.RLock()
	defer certCache.mu.RUnlock()

	allCertKeys := certCache.cacheIndex[name]

	certs := make([]Certificate, len(allCertKeys))
	for i := range allCertKeys {
		certs[i] = certCache.cache[allCertKeys[i]]
	}

	return certs
}

func (certCache *Cache) getAllCerts() []Certificate {
	certCache.mu.RLock()
	defer certCache.mu.RUnlock()
	certs := make([]Certificate, 0, len(certCache.cache))
	for _, cert := range certCache.cache {
		certs = append(certs, cert)
	}
	return certs
}

func (certCache *Cache) getConfig(cert Certificate) (*Config, error) {
	cfg, err := certCache.options.GetConfigForCert(cert)
	if err != nil {
		return nil, err
	}
	if cfg.certCache != nil && cfg.certCache != certCache {
		return nil, fmt.Errorf("config returned for certificate %v is not nil and points to different cache; got %p, expected %p (this one)",
			cert.Names, cfg.certCache, certCache)
	}
	return cfg, nil
}

// AllMatchingCertificates returns a list of all certificates that could
// be used to serve the given SNI name, including exact SAN matches and
// wildcard matches.
func (certCache *Cache) AllMatchingCertificates(name string) []Certificate {
	// get exact matches first
	certs := certCache.getAllMatchingCerts(name)

	// then look for wildcard matches by replacing each
	// label of the domain name with wildcards
	labels := strings.Split(name, ".")
	for i := range labels {
		labels[i] = "*"
		candidate := strings.Join(labels, ".")
		certs = append(certs, certCache.getAllMatchingCerts(candidate)...)
	}

	return certs
}

var (
	defaultCache   *Cache
	defaultCacheMu sync.Mutex
)
