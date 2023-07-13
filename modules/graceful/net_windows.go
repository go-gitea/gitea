// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

//go:build windows

package graceful

import "net"

// GetListener returns a listener from a GetListener function, which must have the
// signature: `func FunctioName(network, address string) (net.Listener, error)`.
// This determines the implementation of net.Listener which the server will use.`
var GetListener = DefaultGetListener

// DefaultGetListener obtains a listener for the local network address.
// On windows this is basically just a shim around net.Listen. This function
// can be replaced by changing the GetListener variable at the top of this file,
// for example to listen on an onion service using github.com/cretz/bine
func DefaultGetListener(network, address string) (net.Listener, error) {
	// Add a deferral to say that we've tried to grab a listener
	defer GetManager().InformCleanup()

	return net.Listen(network, address)
}
