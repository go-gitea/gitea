// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

package graceful

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	listenFDs = "LISTEN_FDS"
	startFD   = 3
)

// In order to keep the working directory the same as when we started we record
// it at startup.
var originalWD, _ = os.Getwd()

var (
	once  = sync.Once{}
	mutex = sync.Mutex{}

	providedListeners = []net.Listener{}
	activeListeners   = []net.Listener{}
)

func getProvidedFDs() (savedErr error) {
	// Only inherit the provided FDS once but we will save the error so that repeated calls to this function will return the same error
	once.Do(func() {
		mutex.Lock()
		defer mutex.Unlock()

		numFDs := os.Getenv(listenFDs)
		if numFDs == "" {
			return
		}
		n, err := strconv.Atoi(numFDs)
		if err != nil {
			savedErr = fmt.Errorf("%s is not a number: %s. Err: %v", listenFDs, numFDs, err)
			return
		}

		for i := startFD; i < n+startFD; i++ {
			file := os.NewFile(uintptr(i), fmt.Sprintf("listener_FD%d", i))

			l, err := net.FileListener(file)
			if err == nil {
				// Close the inherited file if it's a listener
				if err = file.Close(); err != nil {
					savedErr = fmt.Errorf("error closing provided socket fd %d: %s", i, err)
					return
				}
				providedListeners = append(providedListeners, l)
				continue
			}

			// If needed we can handle packetconns here.
			savedErr = fmt.Errorf("Error getting provided socket fd %d: %v", i, err)
			return
		}
	})
	return savedErr
}

// GetListener obtains a listener for the local network address. The network must be
// a stream-oriented network: "tcp", "tcp4", "tcp6", "unix" or "unixpacket". It
// returns an provided net.Listener for the matching network and address, or
// creates a new one using net.Listen.
func GetListener(network, address string) (net.Listener, error) {
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

			activeListeners = append(activeListeners, l)
			return l.(*net.TCPListener), nil
		}
	}

	// no provided listener for this address -> make a fresh listener
	l, err := net.ListenTCP(network, address)
	if err != nil {
		return nil, err
	}
	activeListeners = append(activeListeners, l)
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
			activeListeners = append(activeListeners, l)
			return l.(*net.UnixListener), nil
		}
	}

	// make a fresh listener
	l, err := net.ListenUnix(network, address)
	if err != nil {
		return nil, err
	}
	activeListeners = append(activeListeners, l)
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

// RestartProcess starts a new process passing it the active listeners. It
// doesn't fork, but starts a new process using the same environment and
// arguments as when it was originally started. This allows for a newly
// deployed binary to be started. It returns the pid of the newly started
// process when successful.
func RestartProcess() (int, error) {
	listeners := getActiveListeners()

	// Extract the fds from the listeners.
	files := make([]*os.File, len(listeners))
	for i, l := range listeners {
		var err error
		// Now, all our listeners actually have File() functions so instead of
		// individually casting we just use a hacky interface
		files[i], err = l.(filer).File()
		if err != nil {
			return 0, err
		}
		// Remember to close these at the end.
		defer files[i].Close()
	}

	// Use the original binary location. This works with symlinks such that if
	// the file it points to has been changed we will use the updated symlink.
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return 0, err
	}

	// Pass on the environment and replace the old count key with the new one.
	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, listenFDs+"=") {
			env = append(env, v)
		}
	}
	env = append(env, fmt.Sprintf("%s=%d", listenFDs, len(listeners)))

	allFiles := append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...)
	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   env,
		Files: allFiles,
	})
	if err != nil {
		return 0, err
	}
	return process.Pid, nil
}

type filer interface {
	File() (*os.File, error)
}
