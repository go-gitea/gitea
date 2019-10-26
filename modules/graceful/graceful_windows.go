// +build windows

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// This code is heavily inspired by the archived gofacebook/gracenet/net.go handler

package graceful

// This file contains shims for windows builds
const IsChild = false

// WaitForServers waits for all running servers to finish
func WaitForServers() {

}
