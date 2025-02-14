// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This code is highly inspired by endless go

package graceful

import (
	"crypto/tls"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/proxyprotocol"
	"code.gitea.io/gitea/modules/setting"
)

// GetListener returns a net listener
// This determines the implementation of net.Listener which the server will use,
// so that downstreams could provide their own Listener, such as with a hidden service or a p2p network
var GetListener = DefaultGetListener

// ServeFunction represents a listen.Accept loop
type ServeFunction = func(net.Listener) error

// Server represents our graceful server
type Server struct {
	network              string
	address              string
	listener             net.Listener
	wg                   sync.WaitGroup
	state                state
	lock                 *sync.RWMutex
	BeforeBegin          func(network, address string)
	OnShutdown           func()
	PerWriteTimeout      time.Duration
	PerWritePerKbTimeout time.Duration
}

// NewServer creates a server on network at provided address
func NewServer(network, address, name string) *Server {
	if GetManager().IsChild() {
		log.Info("Restarting new %s server: %s:%s on PID: %d", name, network, address, os.Getpid())
	} else {
		log.Info("Starting new %s server: %s:%s on PID: %d", name, network, address, os.Getpid())
	}
	srv := &Server{
		wg:                   sync.WaitGroup{},
		state:                stateInit,
		lock:                 &sync.RWMutex{},
		network:              network,
		address:              address,
		PerWriteTimeout:      setting.PerWriteTimeout,
		PerWritePerKbTimeout: setting.PerWritePerKbTimeout,
	}

	srv.BeforeBegin = func(network, addr string) {
		log.Debug("Starting server on %s:%s (PID: %d)", network, addr, syscall.Getpid())
	}

	return srv
}

// ListenAndServe listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func (srv *Server) ListenAndServe(serve ServeFunction, useProxyProtocol bool) error {
	go srv.awaitShutdown()

	listener, err := GetListener(srv.network, srv.address)
	if err != nil {
		log.Error("Unable to GetListener: %v", err)
		return err
	}

	// we need to wrap the listener to take account of our lifecycle
	listener = newWrappedListener(listener, srv)

	// Now we need to take account of ProxyProtocol settings...
	if useProxyProtocol {
		listener = &proxyprotocol.Listener{
			Listener:           listener,
			ProxyHeaderTimeout: setting.ProxyProtocolHeaderTimeout,
			AcceptUnknown:      setting.ProxyProtocolAcceptUnknown,
		}
	}
	srv.listener = listener

	srv.BeforeBegin(srv.network, srv.address)

	return srv.Serve(serve)
}

// ListenAndServeTLSConfig listens on the provided network address and then calls
// Serve to handle requests on incoming TLS connections.
func (srv *Server) ListenAndServeTLSConfig(tlsConfig *tls.Config, serve ServeFunction, useProxyProtocol, proxyProtocolTLSBridging bool) error {
	go srv.awaitShutdown()

	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	listener, err := GetListener(srv.network, srv.address)
	if err != nil {
		log.Error("Unable to get Listener: %v", err)
		return err
	}

	// we need to wrap the listener to take account of our lifecycle
	listener = newWrappedListener(listener, srv)

	// Now we need to take account of ProxyProtocol settings... If we're not bridging then we expect that the proxy will forward the connection to us
	if useProxyProtocol && !proxyProtocolTLSBridging {
		listener = &proxyprotocol.Listener{
			Listener:           listener,
			ProxyHeaderTimeout: setting.ProxyProtocolHeaderTimeout,
			AcceptUnknown:      setting.ProxyProtocolAcceptUnknown,
		}
	}

	// Now handle the tls protocol
	listener = tls.NewListener(listener, tlsConfig)

	// Now if we're bridging then we need the proxy to tell us who we're bridging for...
	if useProxyProtocol && proxyProtocolTLSBridging {
		listener = &proxyprotocol.Listener{
			Listener:           listener,
			ProxyHeaderTimeout: setting.ProxyProtocolHeaderTimeout,
			AcceptUnknown:      setting.ProxyProtocolAcceptUnknown,
		}
	}

	srv.listener = listener
	srv.BeforeBegin(srv.network, srv.address)

	return srv.Serve(serve)
}

// Serve accepts incoming HTTP connections on the wrapped listener l, creating a new
// service goroutine for each. The service goroutines read requests and then call
// handler to reply to them. Handler is typically nil, in which case the
// DefaultServeMux is used.
//
// In addition to the standard Serve behaviour each connection is added to a
// sync.Waitgroup so that all outstanding connections can be served before shutting
// down the server.
func (srv *Server) Serve(serve ServeFunction) error {
	defer log.Debug("Serve() returning... (PID: %d)", syscall.Getpid())
	srv.setState(stateRunning)
	GetManager().RegisterServer()
	err := serve(srv.listener)
	log.Debug("Waiting for connections to finish... (PID: %d)", syscall.Getpid())
	srv.wg.Wait()
	srv.setState(stateTerminate)
	GetManager().ServerDone()
	// use of closed means that the listeners are closed - i.e. we should be shutting down - return nil
	if err == nil || strings.Contains(err.Error(), "use of closed") || strings.Contains(err.Error(), "http: Server closed") {
		return nil
	}
	return err
}

func (srv *Server) getState() state {
	srv.lock.RLock()
	defer srv.lock.RUnlock()

	return srv.state
}

func (srv *Server) setState(st state) {
	srv.lock.Lock()
	defer srv.lock.Unlock()

	srv.state = st
}

type filer interface {
	File() (*os.File, error)
}

type wrappedListener struct {
	net.Listener
	stopped bool
	server  *Server
}

func newWrappedListener(l net.Listener, srv *Server) *wrappedListener {
	return &wrappedListener{
		Listener: l,
		server:   srv,
	}
}

func (wl *wrappedListener) Accept() (net.Conn, error) {
	var c net.Conn
	// Set keepalive on TCPListeners connections.
	if tcl, ok := wl.Listener.(*net.TCPListener); ok {
		tc, err := tcl.AcceptTCP()
		if err != nil {
			return nil, err
		}
		_ = tc.SetKeepAlive(true)                  // see http.tcpKeepAliveListener
		_ = tc.SetKeepAlivePeriod(3 * time.Minute) // see http.tcpKeepAliveListener
		c = tc
	} else {
		var err error
		c, err = wl.Listener.Accept()
		if err != nil {
			return nil, err
		}
	}

	closed := int32(0)

	c = &wrappedConn{
		Conn:                 c,
		server:               wl.server,
		closed:               &closed,
		perWriteTimeout:      wl.server.PerWriteTimeout,
		perWritePerKbTimeout: wl.server.PerWritePerKbTimeout,
	}

	wl.server.wg.Add(1)
	return c, nil
}

func (wl *wrappedListener) Close() error {
	if wl.stopped {
		return syscall.EINVAL
	}

	wl.stopped = true
	return wl.Listener.Close()
}

func (wl *wrappedListener) File() (*os.File, error) {
	// returns a dup(2) - FD_CLOEXEC flag *not* set so the listening socket can be passed to child processes
	return wl.Listener.(filer).File()
}

type wrappedConn struct {
	net.Conn
	server               *Server
	closed               *int32
	deadline             time.Time
	perWriteTimeout      time.Duration
	perWritePerKbTimeout time.Duration
}

func (w *wrappedConn) Write(p []byte) (n int, err error) {
	if w.perWriteTimeout > 0 {
		minTimeout := time.Duration(len(p)/1024) * w.perWritePerKbTimeout
		minDeadline := time.Now().Add(minTimeout).Add(w.perWriteTimeout)

		w.deadline = w.deadline.Add(minTimeout)
		if minDeadline.After(w.deadline) {
			w.deadline = minDeadline
		}
		_ = w.Conn.SetWriteDeadline(w.deadline)
	}
	return w.Conn.Write(p)
}

func (w *wrappedConn) Close() error {
	if atomic.CompareAndSwapInt32(w.closed, 0, 1) {
		defer func() {
			if err := recover(); err != nil {
				select {
				case <-GetManager().IsHammer():
					// Likely deadlocked request released at hammertime
					log.Warn("Panic during connection close! %v. Likely there has been a deadlocked request which has been released by forced shutdown.", err)
				default:
					log.Error("Panic during connection close! %v", err)
				}
			}
		}()
		w.server.wg.Done()
	}
	return w.Conn.Close()
}
