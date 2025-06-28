// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"io"
	"strings"

	packages_module "code.gitea.io/gitea/modules/packages"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/sassoftware/go-rpmutils"
)

func SignPackage(buf *packages_module.HashedBuffer, privateKey string) (*packages_module.HashedBuffer, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(privateKey))
	if err != nil {
		return nil, err
	}

	h, err := rpmutils.SignRpmStream(buf, keyring[0].PrivateKey, nil)
	if err != nil {
		return nil, err
	}

	signBlob, err := h.DumpSignatureHeader(false)
	if err != nil {
		return nil, err
	}

	if _, err := buf.Seek(int64(h.OriginalSignatureHeaderSize()), io.SeekStart); err != nil {
		return nil, err
	}

	// create new buf with signature prefix
	return packages_module.CreateHashedBufferFromReader(io.MultiReader(bytes.NewReader(signBlob), buf))
}
