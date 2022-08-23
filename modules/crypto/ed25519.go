// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

// GenerateEd25519Keypair generates a new public and private key from the 25519 curve.
func GenerateEd25519Keypair() (publicKey, privateKey []byte, err error) {
	// Generate the  private key from ed25519.
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ed25519.GenerateKey: %v", err)
	}

	// Marshal the privateKey into the OpenSSH format.
	privPEM, err := marshalPrivateKey(private)
	if err != nil {
		return nil, nil, fmt.Errorf("not able to marshal private key into OpenSSH format: %v", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(public)
	if err != nil {
		return nil, nil, fmt.Errorf("not able to create new SSH public key: %v", err)
	}

	return ssh.MarshalAuthorizedKey(sshPublicKey), pem.EncodeToMemory(privPEM), nil
}

// openSSHMagic contains the magic bytes, which is used to indicate it's a v1
// OpenSSH key format. "openssh-key-v1\x00" in bytes.
const openSSHMagic = "openssh-key-v1\x00"

// MarshalPrivateKey returns a PEM block with the private key serialized in the
// OpenSSH format.
// Adopted from: https://go-review.googlesource.com/c/crypto/+/218620/
func marshalPrivateKey(key ed25519.PrivateKey) (*pem.Block, error) {
	// The ed25519.PrivateKey is a []byte (Seed, Public)

	// Split the provided key in to a public key and private key bytes.
	publicKeyBytes := make([]byte, ed25519.PublicKeySize)
	privateKeyBytes := make([]byte, ed25519.PrivateKeySize)
	copy(publicKeyBytes, key[ed25519.SeedSize:])
	copy(privateKeyBytes, key)

	// Now we want to eventually marshal the sshPrivateKeyStruct below but ssh.Marshal doesn't allow submarshalling
	// So we need to create a number of structs in order to marshal them and build the struct we need.
	//
	// 1. Create a struct that holds the public key for this private key
	pubKeyStruct := struct {
		KeyType string
		Pub     []byte
	}{
		KeyType: ssh.KeyAlgoED25519,
		Pub:     publicKeyBytes,
	}

	// 2. Create a struct to contain the privateKeyBlock
	// 2a. Marshal keypair as the rest struct
	restStruct := struct {
		Pub     []byte
		Priv    []byte
		Comment string
	}{
		publicKeyBytes, privateKeyBytes, "",
	}
	// 2b. Generate a random uint32 number.
	// These can be random bytes or anything else, as long it's the same.
	// See: https://github.com/openssh/openssh-portable/blob/f7fc6a43f1173e8b2c38770bf6cee485a562d03b/sshkey.c#L4228-L4235
	var check uint32
	if err := binary.Read(rand.Reader, binary.BigEndian, &check); err != nil {
		return nil, err
	}

	// 2c. Create the privateKeyBlock struct
	privateKeyBlockStruct := struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Rest    []byte `ssh:"rest"`
	}{
		Check1:  check,
		Check2:  check,
		Keytype: ssh.KeyAlgoED25519,
		Rest:    ssh.Marshal(restStruct),
	}

	// 3. Now we're finally ready to create the OpenSSH sshPrivateKey
	// Head struct of the OpenSSH format.
	sshPrivateKeyStruct := struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte // See pubKey
		PrivKeyBlock []byte // See KeyPair
	}{
		CipherName:   "none", // This is not a password protected key
		KdfName:      "none", // so these fields are left as none and empty
		KdfOpts:      "",     //
		NumKeys:      1,
		PubKey:       ssh.Marshal(pubKeyStruct),
		PrivKeyBlock: generateOpenSSHPadding(ssh.Marshal(privateKeyBlockStruct)),
	}

	// 4. Finally marshal the sshPrivateKeyStruct struct.
	bs := ssh.Marshal(sshPrivateKeyStruct)
	block := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: append([]byte(openSSHMagic), bs...),
	}

	return block, nil
}

// generateOpenSSHPaddins converts the block to
// accomplish a block size of 8 bytes.
func generateOpenSSHPadding(block []byte) []byte {
	padding := []byte{1, 2, 3, 4, 5, 6, 7}

	mod8 := len(block) % 8
	if mod8 > 0 {
		block = append(block, padding[:8-mod8]...)
	}

	return block
}
