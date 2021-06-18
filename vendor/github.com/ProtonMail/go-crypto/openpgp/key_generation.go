// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openpgp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	goerrors "errors"
	"io"
	"math/big"

	"github.com/ProtonMail/go-crypto/openpgp/ecdh"
	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"golang.org/x/crypto/ed25519"
)

// NewEntity returns an Entity that contains a fresh RSA/RSA keypair with a
// single identity composed of the given full name, comment and email, any of
// which may be empty but must not contain any of "()<>\x00".
// If config is nil, sensible defaults will be used.
func NewEntity(name, comment, email string, config *packet.Config) (*Entity, error) {
	creationTime := config.Now()
	keyLifetimeSecs := config.KeyLifetime()

	uid := packet.NewUserId(name, comment, email)
	if uid == nil {
		return nil, errors.InvalidArgumentError("user id field contained invalid characters")
	}

	// Generate a primary signing key
	primaryPrivRaw, err := newSigner(config)
	if err != nil {
		return nil, err
	}
	primary := packet.NewSignerPrivateKey(creationTime, primaryPrivRaw)
	if config != nil && config.V5Keys {
		primary.UpgradeToV5()
	}

	isPrimaryId := true
	selfSignature := &packet.Signature{
		Version:           primary.PublicKey.Version,
		SigType:           packet.SigTypePositiveCert,
		PubKeyAlgo:        primary.PublicKey.PubKeyAlgo,
		Hash:              config.Hash(),
		CreationTime:      creationTime,
		KeyLifetimeSecs:   &keyLifetimeSecs,
		IssuerKeyId:       &primary.PublicKey.KeyId,
		IssuerFingerprint: primary.PublicKey.Fingerprint,
		IsPrimaryId:       &isPrimaryId,
		FlagsValid:        true,
		FlagSign:          true,
		FlagCertify:       true,
		MDC:               true, // true by default, see 5.8 vs. 5.14
		AEAD:              config.AEAD() != nil,
		V5Keys:            config != nil && config.V5Keys,
	}

	// Set the PreferredHash for the SelfSignature from the packet.Config.
	// If it is not the must-implement algorithm from rfc4880bis, append that.
	selfSignature.PreferredHash = []uint8{hashToHashId(config.Hash())}
	if config.Hash() != crypto.SHA256 {
		selfSignature.PreferredHash = append(selfSignature.PreferredHash, hashToHashId(crypto.SHA256))
	}

	// Likewise for DefaultCipher.
	selfSignature.PreferredSymmetric = []uint8{uint8(config.Cipher())}
	if config.Cipher() != packet.CipherAES128 {
		selfSignature.PreferredSymmetric = append(selfSignature.PreferredSymmetric, uint8(packet.CipherAES128))
	}

	// And for DefaultMode.
	selfSignature.PreferredAEAD = []uint8{uint8(config.AEAD().Mode())}
	if config.AEAD().Mode() != packet.AEADModeEAX {
		selfSignature.PreferredAEAD = append(selfSignature.PreferredAEAD, uint8(packet.AEADModeEAX))
	}

	// User ID binding signature
	err = selfSignature.SignUserId(uid.Id, &primary.PublicKey, primary, config)
	if err != nil {
		return nil, err
	}

	// Generate an encryption subkey
	subPrivRaw, err := newDecrypter(config)
	if err != nil {
		return nil, err
	}
	sub := packet.NewDecrypterPrivateKey(creationTime, subPrivRaw)
	sub.IsSubkey = true
	sub.PublicKey.IsSubkey = true
	if config != nil && config.V5Keys {
		sub.UpgradeToV5()
	}

	// NOTE: No KeyLifetimeSecs here, but we will not return this subkey in EncryptionKey()
	// if the primary/master key has expired.
	subKey := Subkey{
		PublicKey:  &sub.PublicKey,
		PrivateKey: sub,
		Sig: &packet.Signature{
			Version:                   primary.PublicKey.Version,
			CreationTime:              creationTime,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                primary.PublicKey.PubKeyAlgo,
			Hash:                      config.Hash(),
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &primary.PublicKey.KeyId,
		},
	}

	// Subkey binding signature
	err = subKey.Sig.SignKey(subKey.PublicKey, primary, config)
	if err != nil {
		return nil, err
	}

	return &Entity{
		PrimaryKey: &primary.PublicKey,
		PrivateKey: primary,
		Identities: map[string]*Identity{
			uid.Id: &Identity{
				Name:          uid.Id,
				UserId:        uid,
				SelfSignature: selfSignature,
				Signatures:    []*packet.Signature{selfSignature},
			},
		},
		Subkeys: []Subkey{subKey},
	}, nil
}

// AddSigningSubkey adds a signing keypair as a subkey to the Entity.
// If config is nil, sensible defaults will be used.
func (e *Entity) AddSigningSubkey(config *packet.Config) error {
	creationTime := config.Now()
	keyLifetimeSecs := config.KeyLifetime()

	subPrivRaw, err := newSigner(config)
	if err != nil {
		return err
	}
	sub := packet.NewSignerPrivateKey(creationTime, subPrivRaw)

	subkey := Subkey{
		PublicKey:  &sub.PublicKey,
		PrivateKey: sub,
		Sig: &packet.Signature{
			Version:         e.PrimaryKey.Version,
			CreationTime:    creationTime,
			KeyLifetimeSecs: &keyLifetimeSecs,
			SigType:         packet.SigTypeSubkeyBinding,
			PubKeyAlgo:      e.PrimaryKey.PubKeyAlgo,
			Hash:            config.Hash(),
			FlagsValid:      true,
			FlagSign:        true,
			IssuerKeyId:     &e.PrimaryKey.KeyId,
			EmbeddedSignature: &packet.Signature{
				Version:      e.PrimaryKey.Version,
				CreationTime: creationTime,
				SigType:      packet.SigTypePrimaryKeyBinding,
				PubKeyAlgo:   sub.PublicKey.PubKeyAlgo,
				Hash:         config.Hash(),
				IssuerKeyId:  &e.PrimaryKey.KeyId,
			},
		},
	}
	if config != nil && config.V5Keys {
		subkey.PublicKey.UpgradeToV5()
	}

	err = subkey.Sig.EmbeddedSignature.CrossSignKey(subkey.PublicKey, e.PrimaryKey, subkey.PrivateKey, config)
	if err != nil {
		return err
	}

	subkey.PublicKey.IsSubkey = true
	subkey.PrivateKey.IsSubkey = true
	if err = subkey.Sig.SignKey(subkey.PublicKey, e.PrivateKey, config); err != nil {
		return err
	}

	e.Subkeys = append(e.Subkeys, subkey)
	return nil
}

// AddEncryptionSubkey adds an encryption keypair as a subkey to the Entity.
// If config is nil, sensible defaults will be used.
func (e *Entity) AddEncryptionSubkey(config *packet.Config) error {
	creationTime := config.Now()
	keyLifetimeSecs := config.KeyLifetime()

	subPrivRaw, err := newDecrypter(config)
	if err != nil {
		return err
	}
	sub := packet.NewDecrypterPrivateKey(creationTime, subPrivRaw)

	subkey := Subkey{
		PublicKey:  &sub.PublicKey,
		PrivateKey: sub,
		Sig: &packet.Signature{
			Version:                   e.PrimaryKey.Version,
			CreationTime:              creationTime,
			KeyLifetimeSecs:           &keyLifetimeSecs,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                e.PrimaryKey.PubKeyAlgo,
			Hash:                      config.Hash(),
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &e.PrimaryKey.KeyId,
		},
	}
	if config != nil && config.V5Keys {
		subkey.PublicKey.UpgradeToV5()
	}

	subkey.PublicKey.IsSubkey = true
	subkey.PrivateKey.IsSubkey = true
	if err = subkey.Sig.SignKey(subkey.PublicKey, e.PrivateKey, config); err != nil {
		return err
	}

	e.Subkeys = append(e.Subkeys, subkey)
	return nil
}

// Generates a signing key
func newSigner(config *packet.Config) (signer crypto.Signer, err error) {
	switch config.PublicKeyAlgorithm() {
	case packet.PubKeyAlgoRSA:
		bits := config.RSAModulusBits()
		if bits < 1024 {
			return nil, errors.InvalidArgumentError("bits must be >= 1024")
		}
		if config != nil && len(config.RSAPrimes) >= 2 {
			primes := config.RSAPrimes[0:2]
			config.RSAPrimes = config.RSAPrimes[2:]
			return generateRSAKeyWithPrimes(config.Random(), 2, bits, primes)
		}
		return rsa.GenerateKey(config.Random(), bits)
	case packet.PubKeyAlgoEdDSA:
		_, priv, err := ed25519.GenerateKey(config.Random())
		if err != nil {
			return nil, err
		}
		return &priv, nil
	default:
		return nil, errors.InvalidArgumentError("unsupported public key algorithm")
	}
}

// Generates an encryption/decryption key
func newDecrypter(config *packet.Config) (decrypter interface{}, err error) {
	switch config.PublicKeyAlgorithm() {
	case packet.PubKeyAlgoRSA:
		bits := config.RSAModulusBits()
		if bits < 1024 {
			return nil, errors.InvalidArgumentError("bits must be >= 1024")
		}
		if config != nil && len(config.RSAPrimes) >= 2 {
			primes := config.RSAPrimes[0:2]
			config.RSAPrimes = config.RSAPrimes[2:]
			return generateRSAKeyWithPrimes(config.Random(), 2, bits, primes)
		}
		return rsa.GenerateKey(config.Random(), bits)
	case packet.PubKeyAlgoEdDSA:
		fallthrough // When passing EdDSA, we generate an ECDH subkey
	case packet.PubKeyAlgoECDH:
		var kdf = ecdh.KDF{
			Hash:   algorithm.SHA512,
			Cipher: algorithm.AES256,
		}
		return ecdh.X25519GenerateKey(config.Random(), kdf)
	default:
		return nil, errors.InvalidArgumentError("unsupported public key algorithm")
	}
}

var bigOne = big.NewInt(1)

// generateRSAKeyWithPrimes generates a multi-prime RSA keypair of the
// given bit size, using the given random source and prepopulated primes.
func generateRSAKeyWithPrimes(random io.Reader, nprimes int, bits int, prepopulatedPrimes []*big.Int) (*rsa.PrivateKey, error) {
	priv := new(rsa.PrivateKey)
	priv.E = 65537

	if nprimes < 2 {
		return nil, goerrors.New("generateRSAKeyWithPrimes: nprimes must be >= 2")
	}

	if bits < 1024 {
		return nil, goerrors.New("generateRSAKeyWithPrimes: bits must be >= 1024")
	}

	primes := make([]*big.Int, nprimes)

NextSetOfPrimes:
	for {
		todo := bits
		// crypto/rand should set the top two bits in each prime.
		// Thus each prime has the form
		//   p_i = 2^bitlen(p_i) × 0.11... (in base 2).
		// And the product is:
		//   P = 2^todo × α
		// where α is the product of nprimes numbers of the form 0.11...
		//
		// If α < 1/2 (which can happen for nprimes > 2), we need to
		// shift todo to compensate for lost bits: the mean value of 0.11...
		// is 7/8, so todo + shift - nprimes * log2(7/8) ~= bits - 1/2
		// will give good results.
		if nprimes >= 7 {
			todo += (nprimes - 2) / 5
		}
		for i := 0; i < nprimes; i++ {
			var err error
			if len(prepopulatedPrimes) == 0 {
				primes[i], err = rand.Prime(random, todo/(nprimes-i))
				if err != nil {
					return nil, err
				}
			} else {
				primes[i] = prepopulatedPrimes[0]
				prepopulatedPrimes = prepopulatedPrimes[1:]
			}

			todo -= primes[i].BitLen()
		}

		// Make sure that primes is pairwise unequal.
		for i, prime := range primes {
			for j := 0; j < i; j++ {
				if prime.Cmp(primes[j]) == 0 {
					continue NextSetOfPrimes
				}
			}
		}

		n := new(big.Int).Set(bigOne)
		totient := new(big.Int).Set(bigOne)
		pminus1 := new(big.Int)
		for _, prime := range primes {
			n.Mul(n, prime)
			pminus1.Sub(prime, bigOne)
			totient.Mul(totient, pminus1)
		}
		if n.BitLen() != bits {
			// This should never happen for nprimes == 2 because
			// crypto/rand should set the top two bits in each prime.
			// For nprimes > 2 we hope it does not happen often.
			continue NextSetOfPrimes
		}

		priv.D = new(big.Int)
		e := big.NewInt(int64(priv.E))
		ok := priv.D.ModInverse(e, totient)

		if ok != nil {
			priv.Primes = primes
			priv.N = n
			break
		}
	}

	priv.Precompute()
	return priv, nil
}
