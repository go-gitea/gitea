// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"

	"code.gitea.io/gitea/modules/log"

	gossh "golang.org/x/crypto/ssh"
)

// GenKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func GenKeyPair(keyPath string) error {
	publicKey, privateKey, err := genKeyPair()
	if err != nil {
		return err
	}

	privKeyFile, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err = privKeyFile.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()
	if _, err := privKeyFile.Write(privateKey); err != nil {
		return err
	}

	pubKeyFile, err := os.OpenFile(keyPath+".pub", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err = pubKeyFile.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()

	_, err = pubKeyFile.Write(publicKey)
	return err
}

func genKeyPair() (publicK, privateK []byte, err error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	privateKeyPEM := &pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: privateKey}
	bufPriv := new(bytes.Buffer)

	if err := pem.Encode(bufPriv, privateKeyPEM); err != nil {
		return nil, nil, err
	}

	// generate public key
	pub, err := gossh.NewPublicKey(&publicKey)
	if err != nil {
		return nil, nil, err
	}

	public := gossh.MarshalAuthorizedKey(pub)

	return bufPriv.Bytes(), public, nil
}
