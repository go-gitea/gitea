// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nosql

import "net/url"

// ToLevelDBURI converts old style connections to a LevelDBURI
//
// A LevelDBURI matches the pattern:
//
// leveldb://path[?[option=value]*]
//
// We have previously just provided the path but this prevent other options
func ToLevelDBURI(connection string) *url.URL {
	uri, err := url.Parse(connection)
	if err == nil && uri.Scheme == "leveldb" {
		return uri
	}
	uri, _ = url.Parse("leveldb://common")
	uri.Host = ""
	uri.Path = connection
	return uri
}
