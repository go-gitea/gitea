package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

func GenerateEd25519KeyPair() (publicKey []byte, privateKey []byte, err error) {
	// Generate the public and private key from ed25519.
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ed25519.GenerateKey: %v", err)
	}

	// Transform private key.

	// Marshal the privateKey into the OpenSSH format.
	privPEM, err := marshalPrivateKey(private)
	if err != nil {
		return nil, nil, fmt.Errorf("not able to marshal private key into OpenSSH format: %v", err)
	}
	// Encode the privateKey into actual bytes.
	privateKey = pem.EncodeToMemory(privPEM)

	// Transform public key.

	// Simply a type conversion to a private ssh's type, which
	// can be used for other ssh functions.
	sshPublicKey, err := ssh.NewPublicKey(public)
	if err != nil {
		return nil, nil, fmt.Errorf("not able to create SSH public key with provided public key: %v", err)
	}

	publicKey = ssh.MarshalAuthorizedKey(sshPublicKey)

	return publicKey, privateKey, nil
}

// openSSHMagic contains the magic bytes, which is used to indicate it's a v1
// OpenSSH key format. "openssh-key-v1\x00" in bytes.
var openSSHMagic = []byte{111, 112, 101, 110, 115, 115, 104, 45, 107, 101, 121, 45, 118, 49, 0}

// MarshalPrivateKey returns a PEM block with the private key serialized in the
// OpenSSH format.
// Adopted from: https://go-review.googlesource.com/c/crypto/+/218620/
func marshalPrivateKey(priv ed25519.PrivateKey) (*pem.Block, error) {
	// Head struct of the OpenSSH format.
	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Struct to represent keypair
	var keyPair struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Rest    []byte `ssh:"rest"`
	}

	// Generate a random uint32 number.
	var check uint32
	if err := binary.Read(rand.Reader, binary.BigEndian, &check); err != nil {
		return nil, err
	}

	// These can be random bytes or anything else, as long it's the same.
	// See: https://github.com/openssh/openssh-portable/blob/f7fc6a43f1173e8b2c38770bf6cee485a562d03b/sshkey.c#L4228-L4235
	keyPair.Check1 = check
	keyPair.Check2 = check

	// Specify the amount of keys it contains.
	w.NumKeys = 1

	// Get the public key from the private key.
	pub := priv[ed25519.PublicKeySize:]

	// Marshal public key.
	pubKey := struct {
		KeyType string
		Pub     []byte
	}{
		ssh.KeyAlgoED25519, pub,
	}
	w.PubKey = ssh.Marshal(pubKey)

	// Marshal keypair.
	privKey := struct {
		Pub  []byte
		Priv []byte
	}{
		pub, priv,
	}
	keyPair.Keytype = ssh.KeyAlgoED25519
	keyPair.Rest = ssh.Marshal(privKey)

	// Interesting part, marshal the keypair and add padding.
	w.PrivKeyBlock = generateOpenSSHPadding(ssh.Marshal(keyPair))

	// We don't use a password protected key,
	// so we don't need to set this to a specific value.
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""

	// Marshal the head struct.
	b := ssh.Marshal(w)
	block := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: append(openSSHMagic, b...),
	}

	return block, nil
}

// generateOpenSSHPaddins converts the block to
// acomplish a block size of 8 bytes.
func generateOpenSSHPadding(block []byte) []byte {
	for i, len := 0, len(block); (len+i)%8 != 0; i++ {
		block = append(block, byte(i+1))
	}
	return block
}
