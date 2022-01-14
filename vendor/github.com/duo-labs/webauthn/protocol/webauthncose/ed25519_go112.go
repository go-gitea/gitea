// +build !go1.13

package webauthncose

import (
	"crypto/x509/pkix"
	"encoding/asn1"

	"golang.org/x/crypto/ed25519"
)

var oidSignatureEd25519 = asn1.ObjectIdentifier{1, 3, 101, 112}

type pkixPublicKey struct {
	Algo      pkix.AlgorithmIdentifier
	BitString asn1.BitString
}

// marshalEd25519PublicKey is a backport of the functionality introduced in
// Go v1.13.
// Ref: https://golang.org/doc/go1.13#crypto/ed25519
// Ref: https://golang.org/doc/go1.13#crypto/x509
func marshalEd25519PublicKey(pub ed25519.PublicKey) ([]byte, error) {
	publicKeyBytes := pub
	var publicKeyAlgorithm pkix.AlgorithmIdentifier
	publicKeyAlgorithm.Algorithm = oidSignatureEd25519

	pkix := pkixPublicKey{
		Algo: publicKeyAlgorithm,
		BitString: asn1.BitString{
			Bytes:     publicKeyBytes,
			BitLength: 8 * len(publicKeyBytes),
		},
	}

	ret, _ := asn1.Marshal(pkix)
	return ret, nil
}
