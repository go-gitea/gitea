// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

//go:build !windows

package graceful

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	listenFDsEnv = "LISTEN_FDS"
	startFD      = 3
	unlinkFDsEnv = "GITEA_UNLINK_FDS"

	notifySocketEnv    = "NOTIFY_SOCKET"
	watchdogTimeoutEnv = "WATCHDOG_USEC"
)

// In order to keep the working directory the same as when we started we record
// it at startup.
var originalWD, _ = os.Getwd()

var (
	once  = sync.Once{}
	mutex = sync.Mutex{}

	providedListenersToUnlink = []bool{}
	activeListenersToUnlink   = []bool{}
	providedListeners         = []net.Listener{}
	activeListeners           = []net.Listener{}

	notifySocketAddr string
	watchdogTimeout  time.Duration
)

func getProvidedFDs() (savedErr error) {
	// Only inherit the provided FDS once but we will save the error so that repeated calls to this function will return the same error
	once.Do(func() {
		mutex.Lock()
		defer mutex.Unlock()
		// now handle some additional systemd provided things
		notifySocketAddr = os.Getenv(notifySocketEnv)
		if notifySocketAddr != "" {
			log.Debug("Systemd Notify Socket provided: %s", notifySocketAddr)
			savedErr = os.Unsetenv(notifySocketEnv)
			if savedErr != nil {
				log.Warn("Unable to Unset the NOTIFY_SOCKET environment variable: %v", savedErr)
				return
			}
			// FIXME: We don't handle WATCHDOG_PID
			timeoutStr := os.Getenv(watchdogTimeoutEnv)
			if timeoutStr != "" {
				savedErr = os.Unsetenv(watchdogTimeoutEnv)
				if savedErr != nil {
					log.Warn("Unable to Unset the WATCHDOG_USEC environment variable: %v", savedErr)
					return
				}

				s, err := strconv.ParseInt(timeoutStr, 10, 64)
				if err != nil {
					log.Error("Unable to parse the provided WATCHDOG_USEC: %v", err)
					savedErr = fmt.Errorf("unable to parse the provided WATCHDOG_USEC: %w", err)
					return
				}
				if s <= 0 {
					log.Error("Unable to parse the provided WATCHDOG_USEC: %s should be a positive number", timeoutStr)
					savedErr = fmt.Errorf("unable to parse the provided WATCHDOG_USEC: %s should be a positive number", timeoutStr)
					return
				}
				watchdogTimeout = time.Duration(s) * time.Microsecond
			}
		} else {
			log.Trace("No Systemd Notify Socket provided")
		}

		numFDs := os.Getenv(listenFDsEnv)
		if numFDs == "" {
			return
		}
		n, err := strconv.Atoi(numFDs)
		if err != nil {
			savedErr = fmt.Errorf("%s is not a number: %s. Err: %w", listenFDsEnv, numFDs, err)
			return
		}

		fdsToUnlinkStr := strings.Split(os.Getenv(unlinkFDsEnv), ",")
		providedListenersToUnlink = make([]bool, n)
		for _, fdStr := range fdsToUnlinkStr {
			i, err := strconv.Atoi(fdStr)
			if err != nil || i < 0 || i >= n {
				continue
			}
			providedListenersToUnlink[i] = true
		}

		for i := startFD; i < n+startFD; i++ {
			file := os.NewFile(uintptr(i), fmt.Sprintf("listener_FD%d", i))

			l, err := net.FileListener(file)
			if err == nil {
				// Close the inherited file if it's a listener
				if err = file.Close(); err != nil {
					savedErr = fmt.Errorf("error closing provided socket fd %d: %w", i, err)
					return
				}
				providedListeners = append(providedListeners, l)
				continue
			}

			// If needed we can handle packetconns here.
			savedErr = fmt.Errorf("Error getting provided socket fd %d: %w", i, err)
			return
		}
	})
	return savedErr
}

// CloseProvidedListeners closes all unused provided listeners.
func CloseProvidedListeners() error {
	mutex.Lock()
	defer mutex.Unlock()
	var returnableError error
	for _, l := range providedListeners {
		err := l.Close()
		if err != nil {
			log.Error("Error in closing unused provided listener: %v", err)
			if returnableError != nil {
				returnableError = fmt.Errorf("%v & %w", returnableError, err)
			} else {
				returnableError = err
			}
		}
	}
	providedListeners = []net.Listener{}

	return returnableError
}

// DefaultGetListener obtains a listener for the stream-oriented local network address:
// "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
func DefaultGetListener(network, address string) (net.Listener, error) {
	// Add a deferral to say that we've tried to grab a listener
	defer GetManager().InformCleanup()
	switch network {
	case "tcp", "tcp4", "tcp6":
		tcpAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, err
		}
		return GetListenerTCP(network, tcpAddr)
	case "unix", "unixpacket":
		unixAddr, err := net.ResolveUnixAddr(network, address)
		if err != nil {
			return nil, err
		}
		return GetListenerUnix(network, unixAddr)
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

// GetListenerTCP announces on the local network address. The network must be:
// "tcp", "tcp4" or "tcp6". It returns a provided net.Listener for the
// matching network and address, or creates a new one using net.ListenTCP.
func GetListenerTCP(network string, address *net.TCPAddr) (*net.TCPListener, error) {
	if err := getProvidedFDs(); err != nil {
		return nil, err
	}

	mutex.Lock()
	defer mutex.Unlock()

	// look for a provided listener
	for i, l := range providedListeners {
		if isSameAddr(l.Addr(), address) {
			providedListeners = append(providedListeners[:i], providedListeners[i+1:]...)
			needsUnlink := providedListenersToUnlink[i]
			providedListenersToUnlink = append(providedListenersToUnlink[:i], providedListenersToUnlink[i+1:]...)

			activeListeners = append(activeListeners, l)
			activeListenersToUnlink = append(activeListenersToUnlink, needsUnlink)
			return l.(*net.TCPListener), nil
		}
	}

	// no provided listener for this address -> make a fresh listener
	l, err := net.ListenTCP(network, address)
	if err != nil {
		return nil, err
	}
	activeListeners = append(activeListeners, l)
	activeListenersToUnlink = append(activeListenersToUnlink, false)
	return l, nil
}

// GetListenerUnix announces on the local network address. The network must be:
// "unix" or "unixpacket". It returns a provided net.Listener for the
// matching network and address, or creates a new one using net.ListenUnix.
func GetListenerUnix(network string, address *net.UnixAddr) (*net.UnixListener, error) {
	if err := getProvidedFDs(); err != nil {
		return nil, err
	}

	mutex.Lock()
	defer mutex.Unlock()

	// look for a provided listener
	for i, l := range providedListeners {
		if isSameAddr(l.Addr(), address) {
			providedListeners = append(providedListeners[:i], providedListeners[i+1:]...)
			needsUnlink := providedListenersToUnlink[i]
			providedListenersToUnlink = append(providedListenersToUnlink[:i], providedListenersToUnlink[i+1:]...)

			activeListenersToUnlink = append(activeListenersToUnlink, needsUnlink)
			activeListeners = append(activeListeners, l)
			unixListener := l.(*net.UnixListener)
			if needsUnlink {
				unixListener.SetUnlinkOnClose(true)
			}
			return unixListener, nil
		}
	}

	// make a fresh listener
	if err := util.Remove(address.Name); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to remove unix socket %s: %w", address.Name, err)
	}

	l, err := net.ListenUnix(network, address)
	if err != nil {
		return nil, err
	}

	fileMode := os.FileMode(setting.UnixSocketPermission)
	if err = os.Chmod(address.Name, fileMode); err != nil {
		return nil, fmt.Errorf("Failed to set permission of unix socket to %s: %w", fileMode.String(), err)
	}

	activeListeners = append(activeListeners, l)
	activeListenersToUnlink = append(activeListenersToUnlink, true)
	return l, nil
}

func isSameAddr(a1, a2 net.Addr) bool {
	// If the addresses are not on the same network fail.
	if a1.Network() != a2.Network() {
		return false
	}

	// If the two addresses have the same string representation they're equal
	a1s := a1.String()
	a2s := a2.String()
	if a1s == a2s {
		return true
	}

	// This allows for ipv6 vs ipv4 local addresses to compare as equal. This
	// scenario is common when listening on localhost.
	const ipv6prefix = "[::]"
	a1s = strings.TrimPrefix(a1s, ipv6prefix)
	a2s = strings.TrimPrefix(a2s, ipv6prefix)
	const ipv4prefix = "0.0.0.0"
	a1s = strings.TrimPrefix(a1s, ipv4prefix)
	a2s = strings.TrimPrefix(a2s, ipv4prefix)
	return a1s == a2s
}

func getActiveListeners() []net.Listener {
	mutex.Lock()
	defer mutex.Unlock()
	listeners := make([]net.Listener, len(activeListeners))
	copy(listeners, activeListeners)
	return listeners
}

func getActiveListenersToUnlink() []bool {
	mutex.Lock()
	defer mutex.Unlock()
	listenersToUnlink := make([]bool, len(activeListenersToUnlink))
	copy(listenersToUnlink, activeListenersToUnlink)
	return listenersToUnlink
}

func getNotifySocket() (*net.UnixConn, error) {
	if err := getProvidedFDs(); err != nil {
		// This error will be logged elsewhere
		return nil, nil
	}

	if notifySocketAddr == "" {
		return nil, nil
	}

	socketAddr := &net.UnixAddr{
		Name: notifySocketAddr,
		Net:  "unixgram",
	}

	notifySocket, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		log.Warn("failed to dial NOTIFY_SOCKET %s: %v", socketAddr, err)
		return nil, err
	}

	return notifySocket, nil
}

func getWatchdogTimeout() time.Duration {
	if err := getProvidedFDs(); err != nil {
		// This error will be logged elsewhere
		return 0
	}

	return watchdogTimeout
}
