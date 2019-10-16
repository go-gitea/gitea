// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import "sync"

var cleanupWaitGroup sync.WaitGroup

func init() {
	cleanupWaitGroup = sync.WaitGroup{}

	// There are three places that could inherit sockets:
	//
	// * HTTP or HTTPS main listener
	// * HTTP redirection fallback
	// * SSH
	//
	// If you add an additional place you must increment this number
	// and add a function to call InformCleanup if it's not going to be used
	cleanupWaitGroup.Add(3)

	// Wait till we're done getting all of the listeners and then close
	// the unused ones
	go func() {
		cleanupWaitGroup.Wait()
		// Ignore the error here there's not much we can do with it
		// They're logged in the CloseProvidedListeners function
		_ = CloseProvidedListeners()
	}()
}

// InformCleanup tells the cleanup wait group that we have either taken a listener
// or will not be taking a listener
func InformCleanup() {
	cleanupWaitGroup.Done()
}
