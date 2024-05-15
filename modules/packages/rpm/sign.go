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

func SignPackage(rpm *packages.HashedBuffer, privateKey string) (io.Reader, int64, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(privateKey)))
	if err != nil {
		// failed to parse key
		return nil, 0, err
	}
	entity := keyring[0]
	h, err := rpmutils.SignRpmStream(rpm, entity.PrivateKey, nil)
	if err != nil {
		// error signing rpm
		return nil, 0, err
	}
	signBlob, err := h.DumpSignatureHeader(false)
	if err != nil {
		// error writing sig header
		return nil, 0, err
	}
	if len(signBlob)%8 != 0 {
		log.Info("incorrect padding: got %d bytes, expected a multiple of 8", len(signBlob))
		return nil, 0, err
	}
	return bytes.NewReader(signBlob), int64(h.OriginalSignatureHeaderSize()), nil
}
