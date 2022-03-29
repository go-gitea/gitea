// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/log"

	"github.com/go-fed/httpsig"
)

// Federation settings
var (
	Federation = struct {
		Enabled         bool
		Algorithms      []string
		DigestAlgorithm string
		GetHeaders      []string
		PostHeaders     []string
	}{
		Enabled:         true,
		Algorithms:      []string{"rsa-sha256", "rsa-sha512"},
		DigestAlgorithm: "SHA-256",
		GetHeaders:      []string{"(request-target)", "Date"},
		PostHeaders:     []string{"(request-target)", "Date", "Digest"},
	}
)

func newFederationService() {
	if err := Cfg.Section("federation").MapTo(&Federation); err != nil {
		log.Fatal("Failed to map Federation settings: %v", err)
	} else if !httpsig.IsSupportedDigestAlgorithm(Federation.DigestAlgorithm) {
		log.Fatal("unsupported digest algorithm: %s", Federation.DigestAlgorithm)
		return
	}
}
