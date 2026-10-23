// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"strings"

	packages_module "gitea.dev/modules/packages"
	rpm_module "gitea.dev/modules/packages/rpm"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func SignPackage(buf *packages_module.HashedBuffer, privateKey string) (*packages_module.HashedBuffer, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privateKey))
	if err != nil {
		return nil, err
	}

	signed, err := rpm_module.SignPackage(buf, keyring[0].PrivateKey)
	if err != nil {
		return nil, err
	}
	return packages_module.CreateHashedBufferFromReader(signed)
}
