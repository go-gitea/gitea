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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	weakrand "math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/mholt/acmez"
	"go.uber.org/zap"
)

// Config configures a certificate manager instance.
// An empty Config is not valid: use New() to obtain
// a valid Config.
type Config struct {
	// How much of a certificate's lifetime becomes the
	// renewal window, which is the span of time at the
	// end of the certificate's validity period in which
	// it should be renewed; for most certificates, the
	// global default is good, but for extremely short-
	// lived certs, you may want to raise this to ~0.5.
	RenewalWindowRatio float64

	// An optional event callback clients can set
	// to subscribe to certain things happening
	// internally by this config; invocations are
	// synchronous, so make them return quickly!
	OnEvent func(event string, data interface{})

	// DefaultServerName specifies a server name
	// to use when choosing a certificate if the
	// ClientHello's ServerName field is empty
	DefaultServerName string

	// The state needed to operate on-demand TLS;
	// if non-nil, on-demand TLS is enabled and
	// certificate operations are deferred to
	// TLS handshakes (or as-needed)
	// TODO: Can we call this feature "Reactive/Lazy/Passive TLS" instead?
	OnDemand *OnDemandConfig

	// Add the must staple TLS extension to the CSR
	MustStaple bool

	// The type that issues certificates; the
	// default Issuer is ACMEManager
	Issuer Issuer

	// The type that revokes certificates; must
	// be configured in conjunction with the Issuer
	// field such that both the Issuer and Revoker
	// are related (because issuance information is
	// required for revocation)
	Revoker Revoker

	// The source of new private keys for certificates;
	// the default KeySource is StandardKeyGenerator
	KeySource KeyGenerator

	// CertSelection chooses one of the certificates
	// with which the ClientHello will be completed;
	// if not set, DefaultCertificateSelector will
	// be used
	CertSelection CertificateSelector

	// The storage to access when storing or
	// loading TLS assets
	Storage Storage

	// Set a logger to enable logging
	Logger *zap.Logger

	// required pointer to the in-memory cert cache
	certCache *Cache
}

// NewDefault makes a valid config based on the package
// Default config. Most users will call this function
// instead of New() since most use cases require only a
// single config for any and all certificates.
//
// If your requirements are more advanced (for example,
// multiple configs depending on the certificate), then use
// New() instead. (You will need to make your own Cache
// first.) If you only need a single Config to manage your
// certs (even if that config changes, as long as it is the
// only one), customize the Default package variable before
// calling NewDefault().
//
// All calls to NewDefault() will return configs that use the
// same, default certificate cache. All configs returned
// by NewDefault() are based on the values of the fields of
// Default at the time it is called.
func NewDefault() *Config {
	defaultCacheMu.Lock()
	if defaultCache == nil {
		defaultCache = NewCache(CacheOptions{
			// the cache will likely need to renew certificates,
			// so it will need to know how to do that, which
			// depends on the certificate being managed and which
			// can change during the lifetime of the cache; this
			// callback makes it possible to get the latest and
			// correct config with which to manage the cert,
			// but if the user does not provide one, we can only
			// assume that we are to use the default config
			GetConfigForCert: func(Certificate) (*Config, error) {
				return NewDefault(), nil
			},
		})
	}
	certCache := defaultCache
	defaultCacheMu.Unlock()

	return newWithCache(certCache, Default)
}

// New makes a new, valid config based on cfg and
// uses the provided certificate cache. certCache
// MUST NOT be nil or this function will panic.
//
// Use this method when you have an advanced use case
// that requires a custom certificate cache and config
// that may differ from the Default. For example, if
// not all certificates are managed/renewed the same
// way, you need to make your own Cache value with a
// GetConfigForCert callback that returns the correct
// configuration for each certificate. However, for
// the vast majority of cases, there will be only a
// single Config, thus the default cache (which always
// uses the default Config) and default config will
// suffice, and you should use New() instead.
func New(certCache *Cache, cfg Config) *Config {
	if certCache == nil {
		panic("a certificate cache is required")
	}
	if certCache.options.GetConfigForCert == nil {
		panic("cache must have GetConfigForCert set in its options")
	}
	return newWithCache(certCache, cfg)
}

// newWithCache ensures that cfg is a valid config by populating
// zero-value fields from the Default Config. If certCache is
// nil, this function panics.
func newWithCache(certCache *Cache, cfg Config) *Config {
	if certCache == nil {
		panic("cannot make a valid config without a pointer to a certificate cache")
	}

	if cfg.OnDemand == nil {
		cfg.OnDemand = Default.OnDemand
	}
	if cfg.RenewalWindowRatio == 0 {
		cfg.RenewalWindowRatio = Default.RenewalWindowRatio
	}
	if cfg.OnEvent == nil {
		cfg.OnEvent = Default.OnEvent
	}
	if cfg.KeySource == nil {
		cfg.KeySource = Default.KeySource
	}
	if cfg.DefaultServerName == "" {
		cfg.DefaultServerName = Default.DefaultServerName
	}
	if cfg.OnDemand == nil {
		cfg.OnDemand = Default.OnDemand
	}
	if !cfg.MustStaple {
		cfg.MustStaple = Default.MustStaple
	}
	if cfg.Storage == nil {
		cfg.Storage = Default.Storage
	}
	if cfg.Issuer == nil {
		cfg.Issuer = Default.Issuer
		if cfg.Issuer == nil {
			// okay really, we need an issuer,
			// that's kind of the point; most
			// people would probably want ACME
			cfg.Issuer = NewACMEManager(&cfg, DefaultACME)
		}
		// issuer and revoker go together; if user
		// specifies their own issuer, we don't want
		// to override their revoker, hence we only
		// do this if Issuer was also nil
		if cfg.Revoker == nil {
			cfg.Revoker = Default.Revoker
			if cfg.Revoker == nil {
				cfg.Revoker = NewACMEManager(&cfg, DefaultACME)
			}
		}
	}

	// absolutely don't allow a nil storage,
	// because that would make almost anything
	// a config can do pointless
	if cfg.Storage == nil {
		cfg.Storage = defaultFileStorage
	}

	// ensure the unexported fields are valid
	cfg.certCache = certCache

	return &cfg
}

// ManageSync causes the certificates for domainNames to be managed
// according to cfg. If cfg.OnDemand is not nil, then this simply
// whitelists the domain names and defers the certificate operations
// to when they are needed. Otherwise, the certificates for each
// name are loaded from storage or obtained from the CA. If loaded
// from storage, they are renewed if they are expiring or expired.
// It then caches the certificate in memory and is prepared to serve
// them up during TLS handshakes.
//
// Note that name whitelisting for on-demand management only takes
// effect if cfg.OnDemand.DecisionFunc is not set (is nil); it will
// not overwrite an existing DecisionFunc, nor will it overwrite
// its decision; i.e. the implicit whitelist is only used if no
// DecisionFunc is set.
//
// This method is synchronous, meaning that certificates for all
// domainNames must be successfully obtained (or renewed) before
// it returns. It returns immediately on the first error for any
// of the given domainNames. This behavior is recommended for
// interactive use (i.e. when an administrator is present) so
// that errors can be reported and fixed immediately.
func (cfg *Config) ManageSync(domainNames []string) error {
	return cfg.manageAll(nil, domainNames, false)
}

// ManageAsync is the same as ManageSync, except that ACME
// operations are performed asynchronously (in the background).
// This method returns before certificates are ready. It is
// crucial that the administrator monitors the logs and is
// notified of any errors so that corrective action can be
// taken as soon as possible. Any errors returned from this
// method occurred before ACME transactions started.
//
// As long as logs are monitored, this method is typically
// recommended for non-interactive environments.
//
// If there are failures loading, obtaining, or renewing a
// certificate, it will be retried with exponential backoff
// for up to about 30 days, with a maximum interval of about
// 24 hours. Cancelling ctx will cancel retries and shut down
// any goroutines spawned by ManageAsync.
func (cfg *Config) ManageAsync(ctx context.Context, domainNames []string) error {
	return cfg.manageAll(ctx, domainNames, true)
}

func (cfg *Config) manageAll(ctx context.Context, domainNames []string, async bool) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, domainName := range domainNames {
		// if on-demand is configured, defer obtain and renew operations
		if cfg.OnDemand != nil {
			if !cfg.OnDemand.whitelistContains(domainName) {
				cfg.OnDemand.hostWhitelist = append(cfg.OnDemand.hostWhitelist, domainName)
			}
			continue
		}

		// otherwise, begin management immediately
		err := cfg.manageOne(ctx, domainName, async)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cfg *Config) manageOne(ctx context.Context, domainName string, async bool) error {
	// first try loading existing certificate from storage
	cert, err := cfg.CacheManagedCertificate(domainName)
	if err != nil {
		if _, ok := err.(ErrNotExist); !ok {
			return fmt.Errorf("%s: caching certificate: %v", domainName, err)
		}
		// if we don't have one in storage, obtain one
		obtain := func() error {
			err := cfg.ObtainCert(ctx, domainName, !async)
			if err != nil {
				return fmt.Errorf("%s: obtaining certificate: %w", domainName, err)
			}
			cert, err = cfg.CacheManagedCertificate(domainName)
			if err != nil {
				return fmt.Errorf("%s: caching certificate after obtaining it: %v", domainName, err)
			}
			return nil
		}
		if async {
			// Leave the job name empty so as to allow duplicate 'obtain'
			// jobs; this is because Caddy calls ManageAsync() before the
			// previous config is stopped (and before its context is
			// canceled), which means that if an obtain job is still
			// running for the same domain, Submit() would not queue the
			// new one because it is still running, even though it is
			// (probably) about to be canceled (it might not if the new
			// config fails to finish loading, however). In any case, we
			// presume it is safe to enqueue a duplicate obtain job because
			// either the old one (or sometimes the new one) is about to be
			// canceled. This seems like reasonable logic for any consumer
			// of this lib. See https://github.com/caddyserver/caddy/issues/3202
			jm.Submit(cfg.Logger, "", obtain)
			return nil
		}
		return obtain()
	}

	// for an existing certificate, make sure it is renewed
	renew := func() error {
		err := cfg.RenewCert(ctx, domainName, !async)
		if err != nil {
			return fmt.Errorf("%s: renewing certificate: %w", domainName, err)
		}
		// successful renewal, so update in-memory cache
		err = cfg.reloadManagedCertificate(cert)
		if err != nil {
			return fmt.Errorf("%s: reloading renewed certificate into memory: %v", domainName, err)
		}
		return nil
	}
	if cert.NeedsRenewal(cfg) {
		if async {
			jm.Submit(cfg.Logger, "renew_"+domainName, renew)
			return nil
		}
		return renew()
	}

	return nil
}

// ObtainCert obtains a certificate for name using cfg, as long
// as a certificate does not already exist in storage for that
// name. The name must qualify and cfg must be flagged as Managed.
// This function is a no-op if storage already has a certificate
// for name.
//
// It only obtains and stores certificates (and their keys),
// it does not load them into memory. If interactive is true,
// the user may be shown a prompt.
// TODO: consider moving interactive param into the Config struct,
// and maybe retry settings into the Config struct as well? (same for RenewCert)
func (cfg *Config) ObtainCert(ctx context.Context, name string, interactive bool) error {
	if cfg.storageHasCertResources(name) {
		return nil
	}
	issuer, err := cfg.getPrecheckedIssuer(ctx, []string{name}, interactive)
	if err != nil {
		return err
	}
	if issuer == nil {
		return nil
	}
	return cfg.obtainWithIssuer(ctx, issuer, name, interactive)
}

func loggerNamed(l *zap.Logger, name string) *zap.Logger {
	if l == nil {
		return nil
	}
	return l.Named(name)
}

func (cfg *Config) obtainWithIssuer(ctx context.Context, issuer Issuer, name string, interactive bool) error {
	log := loggerNamed(cfg.Logger, "obtain")

	if log != nil {
		log.Info("acquiring lock", zap.String("identifier", name))
	}

	// ensure idempotency of the obtain operation for this name
	lockKey := cfg.lockKey("cert_acme", name)
	err := acquireLock(ctx, cfg.Storage, lockKey)
	if err != nil {
		return err
	}
	defer func() {
		if log != nil {
			log.Info("releasing lock", zap.String("identifier", name))
		}
		if err := releaseLock(cfg.Storage, lockKey); err != nil {
			if log != nil {
				log.Error("unable to unlock",
					zap.String("identifier", name),
					zap.String("lock_key", lockKey),
					zap.Error(err))
			}
		}
	}()
	if log != nil {
		log.Info("lock acquired", zap.String("identifier", name))
	}

	f := func(ctx context.Context) error {
		// check if obtain is still needed -- might have been obtained during lock
		if cfg.storageHasCertResources(name) {
			if log != nil {
				log.Info("certificate already exists in storage", zap.String("identifier", name))
			}
			return nil
		}

		privateKey, err := cfg.KeySource.GenerateKey()
		if err != nil {
			return err
		}
		privKeyPEM, err := encodePrivateKey(privateKey)
		if err != nil {
			return err
		}

		csr, err := cfg.generateCSR(privateKey, []string{name})
		if err != nil {
			return err
		}

		issuedCert, err := issuer.Issue(ctx, csr)
		if err != nil {
			return fmt.Errorf("[%s] Obtain: %w", name, err)
		}

		// success - immediately save the certificate resource
		certRes := CertificateResource{
			SANs:           namesFromCSR(csr),
			CertificatePEM: issuedCert.Certificate,
			PrivateKeyPEM:  privKeyPEM,
			IssuerData:     issuedCert.Metadata,
		}
		err = cfg.saveCertResource(certRes)
		if err != nil {
			return fmt.Errorf("[%s] Obtain: saving assets: %v", name, err)
		}

		cfg.emit("cert_obtained", name)

		if log != nil {
			log.Info("certificate obtained successfully", zap.String("identifier", name))
		}

		return nil
	}

	if interactive {
		err = f(ctx)
	} else {
		err = doWithRetry(ctx, log, f)
	}

	return err
}

// RenewCert renews the certificate for name using cfg. It stows the
// renewed certificate and its assets in storage if successful. It
// DOES NOT update the in-memory cache with the new certificate.
func (cfg *Config) RenewCert(ctx context.Context, name string, interactive bool) error {
	issuer, err := cfg.getPrecheckedIssuer(ctx, []string{name}, interactive)
	if err != nil {
		return err
	}
	if issuer == nil {
		return nil
	}
	return cfg.renewWithIssuer(ctx, issuer, name, interactive)
}

func (cfg *Config) renewWithIssuer(ctx context.Context, issuer Issuer, name string, interactive bool) error {
	log := loggerNamed(cfg.Logger, "renew")

	if log != nil {
		log.Info("acquiring lock", zap.String("identifier", name))
	}

	// ensure idempotency of the renew operation for this name
	lockKey := cfg.lockKey("cert_acme", name)
	err := acquireLock(ctx, cfg.Storage, lockKey)
	if err != nil {
		return err
	}
	defer func() {
		if log != nil {
			log.Info("releasing lock", zap.String("identifier", name))
		}
		if err := releaseLock(cfg.Storage, lockKey); err != nil {
			if log != nil {
				log.Error("unable to unlock",
					zap.String("identifier", name),
					zap.String("lock_key", lockKey),
					zap.Error(err))
			}
		}
	}()
	if log != nil {
		log.Info("lock acquired", zap.String("identifier", name))
	}

	f := func(ctx context.Context) error {
		// prepare for renewal (load PEM cert, key, and meta)
		certRes, err := cfg.loadCertResource(name)
		if err != nil {
			return err
		}

		// check if renew is still needed - might have been renewed while waiting for lock
		timeLeft, needsRenew := cfg.managedCertNeedsRenewal(certRes)
		if !needsRenew {
			if log != nil {
				log.Info("certificate appears to have been renewed already",
					zap.String("identifier", name),
					zap.Duration("remaining", timeLeft))
			}
			return nil
		}
		if log != nil {
			log.Info("renewing certificate",
				zap.String("identifier", name),
				zap.Duration("remaining", timeLeft))
		}

		privateKey, err := decodePrivateKey(certRes.PrivateKeyPEM)
		if err != nil {
			return err
		}
		csr, err := cfg.generateCSR(privateKey, []string{name})
		if err != nil {
			return err
		}

		issuedCert, err := issuer.Issue(ctx, csr)
		if err != nil {
			return fmt.Errorf("[%s] Renew: %w", name, err)
		}

		// success - immediately save the renewed certificate resource
		newCertRes := CertificateResource{
			SANs:           namesFromCSR(csr),
			CertificatePEM: issuedCert.Certificate,
			PrivateKeyPEM:  certRes.PrivateKeyPEM,
			IssuerData:     issuedCert.Metadata,
		}
		err = cfg.saveCertResource(newCertRes)
		if err != nil {
			return fmt.Errorf("[%s] Renew: saving assets: %v", name, err)
		}

		cfg.emit("cert_renewed", name)

		if log != nil {
			log.Info("certificate renewed successfully", zap.String("identifier", name))
		}

		return nil
	}

	if interactive {
		err = f(ctx)
	} else {
		err = doWithRetry(ctx, log, f)
	}

	return err
}

func (cfg *Config) generateCSR(privateKey crypto.PrivateKey, sans []string) (*x509.CertificateRequest, error) {
	csrTemplate := new(x509.CertificateRequest)

	for _, name := range sans {
		if ip := net.ParseIP(name); ip != nil {
			csrTemplate.IPAddresses = append(csrTemplate.IPAddresses, ip)
		} else if strings.Contains(name, "@") {
			csrTemplate.EmailAddresses = append(csrTemplate.EmailAddresses, name)
		} else if u, err := url.Parse(name); err == nil && strings.Contains(name, "/") {
			csrTemplate.URIs = append(csrTemplate.URIs, u)
		} else {
			csrTemplate.DNSNames = append(csrTemplate.DNSNames, name)
		}
	}

	if cfg.MustStaple {
		csrTemplate.ExtraExtensions = append(csrTemplate.ExtraExtensions, mustStapleExtension)
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificateRequest(csrDER)
}

// RevokeCert revokes the certificate for domain via ACME protocol. It requires
// that cfg.Issuer is properly configured with the same issuer that issued the
// certificate being revoked. See RFC 5280 §5.3.1 for reason codes.
func (cfg *Config) RevokeCert(ctx context.Context, domain string, reason int, interactive bool) error {
	rev := cfg.Revoker
	if rev == nil {
		rev = Default.Revoker
	}

	certRes, err := cfg.loadCertResource(domain)
	if err != nil {
		return err
	}

	issuerKey := cfg.Issuer.IssuerKey()

	if !cfg.Storage.Exists(StorageKeys.SitePrivateKey(issuerKey, domain)) {
		return fmt.Errorf("private key not found for %s", certRes.SANs)
	}

	err = rev.Revoke(ctx, certRes, reason)
	if err != nil {
		return err
	}

	cfg.emit("cert_revoked", domain)

	err = cfg.Storage.Delete(StorageKeys.SiteCert(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("certificate revoked, but unable to delete certificate file: %v", err)
	}
	err = cfg.Storage.Delete(StorageKeys.SitePrivateKey(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("certificate revoked, but unable to delete private key: %v", err)
	}
	err = cfg.Storage.Delete(StorageKeys.SiteMeta(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("certificate revoked, but unable to delete certificate metadata: %v", err)
	}

	return nil
}

// TLSConfig is an opinionated method that returns a
// recommended, modern TLS configuration that can be
// used to configure TLS listeners, which also supports
// the TLS-ALPN challenge and serves up certificates
// managed by cfg.
//
// Unlike the package TLS() function, this method does
// not, by itself, enable certificate management for
// any domain names.
//
// Feel free to further customize the returned tls.Config,
// but do not mess with the GetCertificate or NextProtos
// fields unless you know what you're doing, as they're
// necessary to solve the TLS-ALPN challenge.
func (cfg *Config) TLSConfig() *tls.Config {
	return &tls.Config{
		// these two fields necessary for TLS-ALPN challenge
		GetCertificate: cfg.GetCertificate,
		NextProtos:     []string{acmez.ACMETLS1Protocol},

		// the rest recommended for modern TLS servers
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites:             preferredDefaultCipherSuites(),
		PreferServerCipherSuites: true,
	}
}

// getPrecheckedIssuer returns an Issuer with pre-checks
// completed, if it is also a PreChecker. It also checks
// that storage is functioning. If a nil Issuer is returned
// with a nil error, that means to skip this operation
// (not an error, just a no-op).
func (cfg *Config) getPrecheckedIssuer(ctx context.Context, names []string, interactive bool) (Issuer, error) {
	// ensure storage is writeable and readable
	// TODO: this is not necessary every time; should only
	// perform check once every so often for each storage,
	// which may require some global state...
	err := cfg.checkStorage()
	if err != nil {
		return nil, fmt.Errorf("failed storage check: %v - storage is probably misconfigured", err)
	}
	if prechecker, ok := cfg.Issuer.(PreChecker); ok {
		err := prechecker.PreCheck(ctx, names, interactive)
		if err != nil {
			return nil, err
		}
	}
	return cfg.Issuer, nil
}

// checkStorage tests the storage by writing random bytes
// to a random key, and then loading those bytes and
// comparing the loaded value. If this fails, the provided
// cfg.Storage mechanism should not be used.
func (cfg *Config) checkStorage() error {
	key := fmt.Sprintf("rw_test_%d", weakrand.Int())
	contents := make([]byte, 1024*10) // size sufficient for one or two ACME resources
	_, err := weakrand.Read(contents)
	if err != nil {
		return err
	}
	err = cfg.Storage.Store(key, contents)
	if err != nil {
		return err
	}
	defer func() {
		deleteErr := cfg.Storage.Delete(key)
		if deleteErr != nil {
			if cfg.Logger != nil {
				cfg.Logger.Error("deleting test key from storage",
					zap.String("key", key), zap.Error(err))
			}
		}
		// if there was no other error, make sure
		// to return any error returned from Delete
		if err == nil {
			err = deleteErr
		}
	}()
	loaded, err := cfg.Storage.Load(key)
	if err != nil {
		return err
	}
	if !bytes.Equal(contents, loaded) {
		return fmt.Errorf("load yielded different value than was stored; expected %d bytes, got %d bytes of differing elements", len(contents), len(loaded))
	}
	return nil
}

// storageHasCertResources returns true if the storage
// associated with cfg's certificate cache has all the
// resources related to the certificate for domain: the
// certificate, the private key, and the metadata.
func (cfg *Config) storageHasCertResources(domain string) bool {
	issuerKey := cfg.Issuer.IssuerKey()
	certKey := StorageKeys.SiteCert(issuerKey, domain)
	keyKey := StorageKeys.SitePrivateKey(issuerKey, domain)
	metaKey := StorageKeys.SiteMeta(issuerKey, domain)
	return cfg.Storage.Exists(certKey) &&
		cfg.Storage.Exists(keyKey) &&
		cfg.Storage.Exists(metaKey)
}

// lockKey returns a key for a lock that is specific to the operation
// named op being performed related to domainName and this config's CA.
func (cfg *Config) lockKey(op, domainName string) string {
	return fmt.Sprintf("%s_%s_%s", op, domainName, cfg.Issuer.IssuerKey())
}

// managedCertNeedsRenewal returns true if certRes is
// expiring soon or already expired, or if the process
// of checking the expiration returned an error.
func (cfg *Config) managedCertNeedsRenewal(certRes CertificateResource) (time.Duration, bool) {
	cert, err := makeCertificate(certRes.CertificatePEM, certRes.PrivateKeyPEM)
	if err != nil {
		return 0, true
	}
	return time.Until(cert.Leaf.NotAfter), cert.NeedsRenewal(cfg)
}

func (cfg *Config) emit(eventName string, data interface{}) {
	if cfg.OnEvent == nil {
		return
	}
	cfg.OnEvent(eventName, data)
}

// CertificateSelector is a type which can select a certificate to use given multiple choices.
type CertificateSelector interface {
	SelectCertificate(*tls.ClientHelloInfo, []Certificate) (Certificate, error)
}

// Constants for PKIX MustStaple extension.
var (
	tlsFeatureExtensionOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 24}
	ocspMustStapleFeature  = []byte{0x30, 0x03, 0x02, 0x01, 0x05}
	mustStapleExtension    = pkix.Extension{
		Id:    tlsFeatureExtensionOID,
		Value: ocspMustStapleFeature,
	}
)
