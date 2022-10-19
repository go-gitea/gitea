// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package proxyprotocol

import "io"

var localHeader = append(v2Prefix, '\x20', '\x00', '\x00', '\x00', '\x00')

// WriteLocalHeader will write the ProxyProtocol Header for a local connection to the provided writer
func WriteLocalHeader(w io.Writer) error {
	_, err := w.Write(localHeader)
	return err
}
