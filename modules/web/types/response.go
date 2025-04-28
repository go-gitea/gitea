// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package types

// ResponseStatusProvider is an interface to get the written status in the response
// Many packages need this interface, so put it in the separate package to avoid import cycle
type ResponseStatusProvider interface {
	WrittenStatus() int
}
