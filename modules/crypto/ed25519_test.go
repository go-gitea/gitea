package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testPublicKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAOhB7/zzhC+HXDdGOdLwJln5NYwm6UNXx3chmQSVTG4\n"
	testPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtz
c2gtZWQyNTUxOQAAACADoQe/884Qvh1w3RjnS8CZZ+TWMJulDV8d3IZkElUxuAAA
AIggISIjICEiIwAAAAtzc2gtZWQyNTUxOQAAACADoQe/884Qvh1w3RjnS8CZZ+TW
MJulDV8d3IZkElUxuAAAAEAAAQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0e
HwOhB7/zzhC+HXDdGOdLwJln5NYwm6UNXx3chmQSVTG4AAAAAAECAwQF
-----END OPENSSH PRIVATE KEY-----
`
)

func TestGeneratingEd25519Keypair(t *testing.T) {
	// Temp override the rand.Reader for deterministic testing.
	oldReader := rand.Reader
	defer func() {
		rand.Reader = oldReader
	}()

	// Only 32 bytes needs to be provided to generate a ed25519 keypair.
	// And another 32 bytes are required, which is included as random value
	// in the OpenSSH format.
	b := make([]byte, 64)
	for i := 0; i < 64; i++ {
		b[i] = byte(i)
	}
	rand.Reader = bytes.NewReader(b)

	publicKey, privateKey, err := GenerateEd25519Keypair()
	assert.NoError(t, err)
	assert.EqualValues(t, testPublicKey, string(publicKey))
	assert.EqualValues(t, testPrivateKey, string(privateKey))
}
