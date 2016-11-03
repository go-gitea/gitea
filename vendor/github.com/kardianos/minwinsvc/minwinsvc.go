// Copyright 2015 Daniel Theophanes.
// Use of this source code is governed by a zlib-style
// license that can be found in the LICENSE file.package service

// Minimal non-invasive windows only service stub.
//
// Import to allow running as a windows service.
//   import _ "github.com/kardianos/minwinsvc"
// This will detect if running as a windows service
// and install required callbacks for windows.
package minwinsvc

// SetOnExit sets the function to be called when the windows service
// requests an exit. If this is not called, or if it is called where
// f == nil, then it defaults to calling "os.Exit(0)".
func SetOnExit(f func()) {
	setOnExit(f)
}
