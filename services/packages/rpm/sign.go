// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/sassoftware/go-rpmutils"
)

func SignPackage(rpm *packages.HashedBuffer, privateKey string) (*packages.HashedBuffer, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(privateKey)))
	if err != nil {
		// failed to parse key
		return nil, err
	}
	entity := keyring[0]
	h, err := rpmutils.SignRpmStream(rpm, entity.PrivateKey, nil)
	if err != nil {
		// error signing rpm
		return nil, err
	}
	signBlob, err := h.DumpSignatureHeader(false)
	if err != nil {
		// error writing sig header
		return nil, err
	}
	if len(signBlob)%8 != 0 {
		log.Info("incorrect padding: got %d bytes, expected a multiple of 8", len(signBlob))
		return nil, err
	}

	// move fp to sign end
	if _, err := rpm.Seek(int64(h.OriginalSignatureHeaderSize()), io.SeekStart); err != nil {
		return nil, err
	}
	// create signed rpm buf
	return packages.CreateHashedBufferFromReader(io.MultiReader(bytes.NewReader(signBlob), rpm))
}
