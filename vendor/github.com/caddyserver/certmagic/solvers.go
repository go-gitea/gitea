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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libdns/libdns"
	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
)

// httpSolver solves the HTTP challenge. It must be
// associated with a config and an address to use
// for solving the challenge. If multiple httpSolvers
// are initialized concurrently, the first one to
// begin will start the server, and the last one to
// finish will stop the server. This solver must be
// wrapped by a distributedSolver to work properly,
// because the only way the HTTP challenge handler
// can access the keyAuth material is by loading it
// from storage, which is done by distributedSolver.
type httpSolver struct {
	closed      int32 // accessed atomically
	acmeManager *ACMEManager
	address     string
}

// Present starts an HTTP server if none is already listening on s.address.
func (s *httpSolver) Present(ctx context.Context, _ acme.Challenge) error {
	solversMu.Lock()
	defer solversMu.Unlock()

	si := getSolverInfo(s.address)
	si.count++
	if si.listener != nil {
		return nil // already be served by us
	}

	// notice the unusual error handling here; we
	// only continue to start a challenge server if
	// we got a listener; in all other cases return
	ln, err := robustTryListen(s.address)
	if ln == nil {
		return err
	}

	// successfully bound socket, so save listener and start key auth HTTP server
	si.listener = ln
	go s.serve(si)

	return nil
}

// serve is an HTTP server that serves only HTTP challenge responses.
func (s *httpSolver) serve(si *solverInfo) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic: http solver server: %v\n%s", err, buf)
		}
	}()
	defer close(si.done)
	httpServer := &http.Server{Handler: s.acmeManager.HTTPChallengeHandler(http.NewServeMux())}
	httpServer.SetKeepAlivesEnabled(false)
	err := httpServer.Serve(si.listener)
	if err != nil && atomic.LoadInt32(&s.closed) != 1 {
		log.Printf("[ERROR] key auth HTTP server: %v", err)
	}
}

// CleanUp cleans up the HTTP server if it is the last one to finish.
func (s *httpSolver) CleanUp(ctx context.Context, _ acme.Challenge) error {
	solversMu.Lock()
	defer solversMu.Unlock()
	si := getSolverInfo(s.address)
	si.count--
	if si.count == 0 {
		// last one out turns off the lights
		atomic.StoreInt32(&s.closed, 1)
		if si.listener != nil {
			si.listener.Close()
			<-si.done
		}
		delete(solvers, s.address)
	}
	return nil
}

// tlsALPNSolver is a type that can solve TLS-ALPN challenges.
// It must have an associated config and address on which to
// serve the challenge.
type tlsALPNSolver struct {
	config  *Config
	address string
}

// Present adds the certificate to the certificate cache and, if
// needed, starts a TLS server for answering TLS-ALPN challenges.
func (s *tlsALPNSolver) Present(ctx context.Context, chal acme.Challenge) error {
	// we pre-generate the certificate for efficiency with multi-perspective
	// validation, so it only has to be done once (at least, by this instance;
	// distributed solving does not have that luxury, oh well) - update the
	// challenge data in memory to be the generated certificate
	cert, err := acmez.TLSALPN01ChallengeCert(chal)
	if err != nil {
		return err
	}
	activeChallengesMu.Lock()
	chalData := activeChallenges[chal.Identifier.Value]
	chalData.data = cert
	activeChallenges[chal.Identifier.Value] = chalData
	activeChallengesMu.Unlock()

	// the rest of this function increments the
	// challenge count for the solver at this
	// listener address, and if necessary, starts
	// a simple TLS server

	solversMu.Lock()
	defer solversMu.Unlock()

	si := getSolverInfo(s.address)
	si.count++
	if si.listener != nil {
		return nil // already be served by us
	}

	// notice the unusual error handling here; we
	// only continue to start a challenge server if
	// we got a listener; in all other cases return
	ln, err := robustTryListen(s.address)
	if ln == nil {
		return err
	}

	// we were able to bind the socket, so make it into a TLS
	// listener, store it with the solverInfo, and start the
	// challenge server

	si.listener = tls.NewListener(ln, s.config.TLSConfig())

	go func() {
		defer func() {
			if err := recover(); err != nil {
				buf := make([]byte, stackTraceBufferSize)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("panic: tls-alpn solver server: %v\n%s", err, buf)
			}
		}()
		defer close(si.done)
		for {
			conn, err := si.listener.Accept()
			if err != nil {
				if atomic.LoadInt32(&si.closed) == 1 {
					return
				}
				log.Printf("[ERROR] TLS-ALPN challenge server: accept: %v", err)
				continue
			}
			go s.handleConn(conn)
		}
	}()

	return nil
}

// handleConn completes the TLS handshake and then closes conn.
func (*tlsALPNSolver) handleConn(conn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, stackTraceBufferSize)
			buf = buf[:runtime.Stack(buf, false)]
			log.Printf("panic: tls-alpn solver handler: %v\n%s", err, buf)
		}
	}()
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		log.Printf("[ERROR] TLS-ALPN challenge server: expected tls.Conn but got %T: %#v", conn, conn)
		return
	}
	err := tlsConn.Handshake()
	if err != nil {
		log.Printf("[ERROR] TLS-ALPN challenge server: handshake: %v", err)
		return
	}
}

// CleanUp removes the challenge certificate from the cache, and if
// it is the last one to finish, stops the TLS server.
func (s *tlsALPNSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	s.config.certCache.mu.Lock()
	delete(s.config.certCache.cache, tlsALPNCertKeyName(chal.Identifier.Value))
	s.config.certCache.mu.Unlock()

	solversMu.Lock()
	defer solversMu.Unlock()
	si := getSolverInfo(s.address)
	si.count--
	if si.count == 0 {
		// last one out turns off the lights
		atomic.StoreInt32(&si.closed, 1)
		if si.listener != nil {
			si.listener.Close()
			<-si.done
		}
		delete(solvers, s.address)
	}

	return nil
}

// tlsALPNCertKeyName returns the key to use when caching a cert
// for use with the TLS-ALPN ACME challenge. It is simply to help
// avoid conflicts (although at time of writing, there shouldn't
// be, since the cert cache is keyed by hash of certificate chain).
func tlsALPNCertKeyName(sniName string) string {
	return sniName + ":acme-tls-alpn"
}

// DNS01Solver is a type that makes libdns providers usable
// as ACME dns-01 challenge solvers.
// See https://github.com/libdns/libdns
type DNS01Solver struct {
	// The implementation that interacts with the DNS
	// provider to set or delete records. (REQUIRED)
	DNSProvider ACMEDNSProvider

	// The TTL for the temporary challenge records.
	TTL time.Duration

	// Maximum time to wait for temporary record to appear.
	PropagationTimeout time.Duration

	// Preferred DNS resolver(s) to use when doing DNS lookups.
	Resolvers []string

	txtRecords   map[string]dnsPresentMemory // keyed by domain name
	txtRecordsMu sync.Mutex
}

// Present creates the DNS TXT record for the given ACME challenge.
func (s *DNS01Solver) Present(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	keyAuth := challenge.DNS01KeyAuthorization()

	// multiple identifiers can have the same ACME challenge
	// domain (e.g. example.com and *.example.com) so we need
	// to ensure that we don't solve those concurrently and
	// step on each challenges' metaphorical toes; see
	// https://github.com/caddyserver/caddy/issues/3474
	activeDNSChallenges.Lock(dnsName)

	zone, err := findZoneByFQDN(dnsName, recursiveNameservers(s.Resolvers))
	if err != nil {
		return fmt.Errorf("could not determine zone for domain %q: %v", dnsName, err)
	}

	rec := libdns.Record{
		Type:  "TXT",
		Name:  libdns.RelativeName(dnsName+".", zone),
		Value: keyAuth,
		TTL:   s.TTL,
	}

	results, err := s.DNSProvider.AppendRecords(ctx, zone, []libdns.Record{rec})
	if err != nil {
		return fmt.Errorf("adding temporary record for zone %s: %w", zone, err)
	}
	if len(results) != 1 {
		return fmt.Errorf("expected one record, got %d: %v", len(results), results)
	}

	// remember the record and zone we got so we can clean up more efficiently
	s.txtRecordsMu.Lock()
	if s.txtRecords == nil {
		s.txtRecords = make(map[string]dnsPresentMemory)
	}
	s.txtRecords[dnsName] = dnsPresentMemory{dnsZone: zone, rec: results[0]}
	s.txtRecordsMu.Unlock()

	return nil
}

// Wait blocks until the TXT record created in Present() appears in
// authoritative lookups, i.e. until it has propagated, or until
// timeout, whichever is first.
func (s *DNS01Solver) Wait(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()
	keyAuth := challenge.DNS01KeyAuthorization()

	timeout := s.PropagationTimeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	const interval = 2 * time.Second

	resolvers := recursiveNameservers(s.Resolvers)

	var err error
	start := time.Now()
	for time.Since(start) < timeout {
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
		var ready bool
		ready, err = checkDNSPropagation(dnsName, keyAuth, resolvers)
		if err != nil {
			return fmt.Errorf("checking DNS propagation of %s: %w", dnsName, err)
		}
		if ready {
			return nil
		}
	}

	return fmt.Errorf("timed out waiting for record to fully propagate; verify DNS provider configuration is correct - last error: %v", err)
}

// CleanUp deletes the DNS TXT record created in Present().
func (s *DNS01Solver) CleanUp(ctx context.Context, challenge acme.Challenge) error {
	dnsName := challenge.DNS01TXTRecordName()

	defer func() {
		// always forget about it so we don't leak memory
		s.txtRecordsMu.Lock()
		delete(s.txtRecords, dnsName)
		s.txtRecordsMu.Unlock()

		// always do this last - but always do it!
		activeDNSChallenges.Unlock(dnsName)
	}()

	// recall the record we created and zone we looked up
	s.txtRecordsMu.Lock()
	memory, ok := s.txtRecords[dnsName]
	if !ok {
		s.txtRecordsMu.Unlock()
		return fmt.Errorf("no memory of presenting a DNS record for %s (probably OK if presenting failed)", challenge.Identifier.Value)
	}
	s.txtRecordsMu.Unlock()

	// clean up the record
	_, err := s.DNSProvider.DeleteRecords(ctx, memory.dnsZone, []libdns.Record{memory.rec})
	if err != nil {
		return fmt.Errorf("deleting temporary record for zone %s: %w", memory.dnsZone, err)
	}

	return nil
}

type dnsPresentMemory struct {
	dnsZone string
	rec     libdns.Record
}

// ACMEDNSProvider defines the set of operations required for
// ACME challenges. A DNS provider must be able to append and
// delete records in order to solve ACME challenges. Find one
// you can use at https://github.com/libdns. If your provider
// isn't implemented yet, feel free to contribute!
type ACMEDNSProvider interface {
	libdns.RecordAppender
	libdns.RecordDeleter
}

// activeDNSChallenges synchronizes DNS challenges for
// names to ensure that challenges for the same ACME
// DNS name do not overlap; for example, the TXT record
// to make for both example.com and *.example.com are
// the same; thus we cannot solve them concurrently.
var activeDNSChallenges = newMapMutex()

// mapMutex implements named mutexes.
type mapMutex struct {
	cond *sync.Cond
	set  map[interface{}]struct{}
}

func newMapMutex() *mapMutex {
	return &mapMutex{
		cond: sync.NewCond(new(sync.Mutex)),
		set:  make(map[interface{}]struct{}),
	}
}

func (mmu *mapMutex) Lock(key interface{}) {
	mmu.cond.L.Lock()
	defer mmu.cond.L.Unlock()
	for mmu.locked(key) {
		mmu.cond.Wait()
	}
	mmu.set[key] = struct{}{}
	return
}

func (mmu *mapMutex) Unlock(key interface{}) {
	mmu.cond.L.Lock()
	defer mmu.cond.L.Unlock()
	delete(mmu.set, key)
	mmu.cond.Broadcast()
}

func (mmu *mapMutex) locked(key interface{}) (ok bool) {
	_, ok = mmu.set[key]
	return
}

// distributedSolver allows the ACME HTTP-01 and TLS-ALPN challenges
// to be solved by an instance other than the one which initiated it.
// This is useful behind load balancers or in other cluster/fleet
// configurations. The only requirement is that the instance which
// initiates the challenge shares the same storage and locker with
// the others in the cluster. The storage backing the certificate
// cache in distributedSolver.config is crucial.
//
// Obviously, the instance which completes the challenge must be
// serving on the HTTPChallengePort for the HTTP-01 challenge or the
// TLSALPNChallengePort for the TLS-ALPN-01 challenge (or have all
// the packets port-forwarded) to receive and handle the request. The
// server which receives the challenge must handle it by checking to
// see if the challenge token exists in storage, and if so, decode it
// and use it to serve up the correct response. HTTPChallengeHandler
// in this package as well as the GetCertificate method implemented
// by a Config support and even require this behavior.
//
// In short: the only two requirements for cluster operation are
// sharing sync and storage, and using the facilities provided by
// this package for solving the challenges.
type distributedSolver struct {
	// The storage backing the distributed solver. It must be
	// the same storage configuration as what is solving the
	// challenge in order to be effective.
	storage Storage

	// The storage key prefix, associated with the issuer
	// that is solving the challenge.
	storageKeyIssuerPrefix string

	// Since the distributedSolver is only a
	// wrapper over an actual solver, place
	// the actual solver here.
	solver acmez.Solver
}

// Present invokes the underlying solver's Present method
// and also stores domain, token, and keyAuth to the storage
// backing the certificate cache of dhs.acmeManager.
func (dhs distributedSolver) Present(ctx context.Context, chal acme.Challenge) error {
	infoBytes, err := json.Marshal(chal)
	if err != nil {
		return err
	}

	err = dhs.storage.Store(dhs.challengeTokensKey(chal.Identifier.Value), infoBytes)
	if err != nil {
		return err
	}

	err = dhs.solver.Present(ctx, chal)
	if err != nil {
		return fmt.Errorf("presenting with embedded solver: %v", err)
	}
	return nil
}

// Wait wraps the underlying solver's Wait() method, if any. Implements acmez.Waiter.
func (dhs distributedSolver) Wait(ctx context.Context, challenge acme.Challenge) error {
	if waiter, ok := dhs.solver.(acmez.Waiter); ok {
		return waiter.Wait(ctx, challenge)
	}
	return nil
}

// CleanUp invokes the underlying solver's CleanUp method
// and also cleans up any assets saved to storage.
func (dhs distributedSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	err := dhs.storage.Delete(dhs.challengeTokensKey(chal.Identifier.Value))
	if err != nil {
		return err
	}
	err = dhs.solver.CleanUp(ctx, chal)
	if err != nil {
		return fmt.Errorf("cleaning up embedded provider: %v", err)
	}
	return nil
}

// challengeTokensPrefix returns the key prefix for challenge info.
func (dhs distributedSolver) challengeTokensPrefix() string {
	return path.Join(dhs.storageKeyIssuerPrefix, "challenge_tokens")
}

// challengeTokensKey returns the key to use to store and access
// challenge info for domain.
func (dhs distributedSolver) challengeTokensKey(domain string) string {
	return path.Join(dhs.challengeTokensPrefix(), StorageKeys.Safe(domain)+".json")
}

// solverInfo associates a listener with the
// number of challenges currently using it.
type solverInfo struct {
	closed   int32 // accessed atomically
	count    int
	listener net.Listener
	done     chan struct{} // used to signal when our own solver server is done
}

// getSolverInfo gets a valid solverInfo struct for address.
func getSolverInfo(address string) *solverInfo {
	si, ok := solvers[address]
	if !ok {
		si = &solverInfo{done: make(chan struct{})}
		solvers[address] = si
	}
	return si
}

// robustTryListen calls net.Listen for a TCP socket at addr.
// This function may return both a nil listener and a nil error!
// If it was able to bind the socket, it returns the listener
// and no error. If it wasn't able to bind the socket because
// the socket is already in use, then it returns a nil listener
// and nil error. If it had any other error, it returns the
// error. The intended error handling logic for this function
// is to proceed if the returned listener is not nil; otherwise
// return err (which may also be nil). In other words, this
// function ignores errors if the socket is already in use,
// which is useful for our challenge servers, where we assume
// that whatever is already listening can solve the challenges.
func robustTryListen(addr string) (net.Listener, error) {
	var listenErr error
	for i := 0; i < 2; i++ {
		// doesn't hurt to sleep briefly before the second
		// attempt in case the OS has timing issues
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		// if we can bind the socket right away, great!
		var ln net.Listener
		ln, listenErr = net.Listen("tcp", addr)
		if listenErr == nil {
			return ln, nil
		}

		// if it failed just because the socket is already in use, we
		// have no choice but to assume that whatever is using the socket
		// can answer the challenge already, so we ignore the error
		connectErr := dialTCPSocket(addr)
		if connectErr == nil {
			return nil, nil
		}

		// hmm, we couldn't connect to the socket, so something else must
		// be wrong, right? wrong!! we've had reports across multiple OSes
		// now that sometimes connections fail even though the OS told us
		// that the address was already in use; either the listener is
		// fluctuating between open and closed very, very quickly, or the
		// OS is inconsistent and contradicting itself; I have been unable
		// to reproduce this, so I'm now resorting to hard-coding substring
		// matching in error messages as a really hacky and unreliable
		// safeguard against this, until we can idenify exactly what was
		// happening; see the following threads for more info:
		// https://caddy.community/t/caddy-retry-error/7317
		// https://caddy.community/t/v2-upgrade-to-caddy2-failing-with-errors/7423
		if strings.Contains(listenErr.Error(), "address already in use") ||
			strings.Contains(listenErr.Error(), "one usage of each socket address") {
			log.Printf("[WARNING] OS reports a contradiction: %v - but we cannot connect to it, with this error: %v; continuing anyway 🤞 (I don't know what causes this... if you do, please help?)", listenErr, connectErr)
			return nil, nil
		}
	}
	return nil, fmt.Errorf("could not start listener for challenge server at %s: %v", addr, listenErr)
}

// dialTCPSocket connects to a TCP address just for the sake of
// seeing if it is open. It returns a nil error if a TCP connection
// can successfully be made to addr within a short timeout.
func dialTCPSocket(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
	if err == nil {
		conn.Close()
	}
	return err
}

// GetACMEChallenge returns an active ACME challenge for the given identifier,
// or false if no active challenge for that identifier is known.
func GetACMEChallenge(identifier string) (Challenge, bool) {
	activeChallengesMu.Lock()
	chalData, ok := activeChallenges[identifier]
	activeChallengesMu.Unlock()
	return chalData, ok
}

// The active challenge solvers, keyed by listener address,
// and protected by a mutex. Note that the creation of
// solver listeners and the incrementing of their counts
// are atomic operations guarded by this mutex.
var (
	solvers   = make(map[string]*solverInfo)
	solversMu sync.Mutex
)

// activeChallenges holds information about all known, currently-active
// ACME challenges, keyed by identifier. CertMagic guarantees that
// challenges for the same identifier do not overlap, by its locking
// mechanisms; thus if a challenge comes in for a certain identifier,
// we can be confident that if this process initiated the challenge,
// the correct information to solve it is in this map. (It may have
// alternatively been initiated by another instance in a cluster, in
// which case the distributed solver will take care of that.)
var (
	activeChallenges   = make(map[string]Challenge)
	activeChallengesMu sync.Mutex
)

// Challenge is an ACME challenge, but optionally paired with
// data that can make it easier or more efficient to solve.
type Challenge struct {
	acme.Challenge
	data interface{}
}

// solverWrapper should be used to wrap all challenge solvers so that
// we can add the challenge info to memory; this makes challenges globally
// solvable by a single HTTP or TLS server even if multiple servers with
// different configurations/scopes need to get certificates.
type solverWrapper struct{ acmez.Solver }

func (sw solverWrapper) Present(ctx context.Context, chal acme.Challenge) error {
	activeChallengesMu.Lock()
	activeChallenges[chal.Identifier.Value] = Challenge{Challenge: chal}
	activeChallengesMu.Unlock()
	return sw.Solver.Present(ctx, chal)
}

func (sw solverWrapper) Wait(ctx context.Context, chal acme.Challenge) error {
	if waiter, ok := sw.Solver.(acmez.Waiter); ok {
		return waiter.Wait(ctx, chal)
	}
	return nil
}

func (sw solverWrapper) CleanUp(ctx context.Context, chal acme.Challenge) error {
	activeChallengesMu.Lock()
	delete(activeChallenges, chal.Identifier.Value)
	activeChallengesMu.Unlock()
	return sw.Solver.CleanUp(ctx, chal)
}

// Interface guards
var (
	_ acmez.Solver = (*solverWrapper)(nil)
	_ acmez.Waiter = (*solverWrapper)(nil)
	_ acmez.Waiter = (*distributedSolver)(nil)
)
