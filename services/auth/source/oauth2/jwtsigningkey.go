// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidAlgorithmType represents an invalid algorithm error.
type ErrInvalidAlgorithmType struct {
	Algorithm string
}

func (err ErrInvalidAlgorithmType) Error() string {
	return fmt.Sprintf("JWT signing algorithm is not supported: %s", err.Algorithm)
}

// JWTSigningKey represents a algorithm/key pair to sign JWTs
type JWTSigningKey interface {
	IsSymmetric() bool
	SigningMethod() jwt.SigningMethod
	SignKey() any
	VerifyKey() any
	ToJWK() (map[string]string, error)
	PreProcessToken(*jwt.Token)
}

type hmacSigningKey struct {
	signingMethod jwt.SigningMethod
	secret        []byte
}

func (key hmacSigningKey) IsSymmetric() bool {
	return true
}

func (key hmacSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key hmacSigningKey) SignKey() any {
	return key.secret
}

func (key hmacSigningKey) VerifyKey() any {
	return key.secret
}

func (key hmacSigningKey) ToJWK() (map[string]string, error) {
	return map[string]string{
		"kty": "oct",
		"alg": key.SigningMethod().Alg(),
	}, nil
}

func (key hmacSigningKey) PreProcessToken(*jwt.Token) {}

type rsaSingingKey struct {
	signingMethod jwt.SigningMethod
	key           *rsa.PrivateKey
	id            string
}

func newRSASingingKey(signingMethod jwt.SigningMethod, key *rsa.PrivateKey) (rsaSingingKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(*rsa.PublicKey))
	if err != nil {
		return rsaSingingKey{}, err
	}

	return rsaSingingKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key rsaSingingKey) IsSymmetric() bool {
	return false
}

func (key rsaSingingKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key rsaSingingKey) SignKey() any {
	return key.key
}

func (key rsaSingingKey) VerifyKey() any {
	return key.key.Public()
}

func (key rsaSingingKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(*rsa.PublicKey)

	return map[string]string{
		"kty": "RSA",
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
		"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
	}, nil
}

func (key rsaSingingKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

type eddsaSigningKey struct {
	signingMethod jwt.SigningMethod
	key           ed25519.PrivateKey
	id            string
}

func newEdDSASingingKey(signingMethod jwt.SigningMethod, key ed25519.PrivateKey) (eddsaSigningKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(ed25519.PublicKey))
	if err != nil {
		return eddsaSigningKey{}, err
	}

	return eddsaSigningKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key eddsaSigningKey) IsSymmetric() bool {
	return false
}

func (key eddsaSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key eddsaSigningKey) SignKey() any {
	return key.key
}

func (key eddsaSigningKey) VerifyKey() any {
	return key.key.Public()
}

func (key eddsaSigningKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(ed25519.PublicKey)

	return map[string]string{
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pubKey),
	}, nil
}

func (key eddsaSigningKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

type ecdsaSingingKey struct {
	signingMethod jwt.SigningMethod
	key           *ecdsa.PrivateKey
	id            string
}

func newECDSASingingKey(signingMethod jwt.SigningMethod, key *ecdsa.PrivateKey) (ecdsaSingingKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(*ecdsa.PublicKey))
	if err != nil {
		return ecdsaSingingKey{}, err
	}

	return ecdsaSingingKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key ecdsaSingingKey) IsSymmetric() bool {
	return false
}

func (key ecdsaSingingKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key ecdsaSingingKey) SignKey() any {
	return key.key
}

func (key ecdsaSingingKey) VerifyKey() any {
	return key.key.Public()
}

func (key ecdsaSingingKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(*ecdsa.PublicKey)

	return map[string]string{
		"kty": "EC",
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"crv": pubKey.Params().Name,
		"x":   base64.RawURLEncoding.EncodeToString(pubKey.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(pubKey.Y.Bytes()),
	}, nil
}

func (key ecdsaSingingKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

// CreateJWTSigningKey creates a signing key from an algorithm / key pair.
func CreateJWTSigningKey(algorithm string, key any) (JWTSigningKey, error) {
	var signingMethod jwt.SigningMethod
	switch algorithm {
	case "HS256":
		signingMethod = jwt.SigningMethodHS256
	case "HS384":
		signingMethod = jwt.SigningMethodHS384
	case "HS512":
		signingMethod = jwt.SigningMethodHS512

	case "RS256":
		signingMethod = jwt.SigningMethodRS256
	case "RS384":
		signingMethod = jwt.SigningMethodRS384
	case "RS512":
		signingMethod = jwt.SigningMethodRS512

	case "ES256":
		signingMethod = jwt.SigningMethodES256
	case "ES384":
		signingMethod = jwt.SigningMethodES384
	case "ES512":
		signingMethod = jwt.SigningMethodES512
	case "EdDSA":
		signingMethod = jwt.SigningMethodEdDSA
	default:
		return nil, ErrInvalidAlgorithmType{algorithm}
	}

	switch signingMethod.(type) {
	case *jwt.SigningMethodEd25519:
		privateKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newEdDSASingingKey(signingMethod, privateKey)
	case *jwt.SigningMethodECDSA:
		privateKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newECDSASingingKey(signingMethod, privateKey)
	case *jwt.SigningMethodRSA:
		privateKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newRSASingingKey(signingMethod, privateKey)
	default:
		secret, ok := key.([]byte)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return hmacSigningKey{signingMethod, secret}, nil
	}
}

// DefaultSigningKey is the default signing key for JWTs.
var DefaultSigningKey JWTSigningKey

// InitSigningKey creates the default signing key from settings or creates a random key.
func InitSigningKey() error {
	var err error
	var key any

	switch setting.OAuth2.JWTSigningAlgorithm {
	case "HS256":
		fallthrough
	case "HS384":
		fallthrough
	case "HS512":
		key, err = loadSymmetricKey()
	case "RS256":
		fallthrough
	case "RS384":
		fallthrough
	case "RS512":
		fallthrough
	case "ES256":
		fallthrough
	case "ES384":
		fallthrough
	case "ES512":
		fallthrough
	case "EdDSA":
		key, err = loadOrCreateAsymmetricKey()
	default:
		return ErrInvalidAlgorithmType{setting.OAuth2.JWTSigningAlgorithm}
	}

	if err != nil {
		return fmt.Errorf("Error while loading or creating JWT key: %w", err)
	}

	signingKey, err := CreateJWTSigningKey(setting.OAuth2.JWTSigningAlgorithm, key)
	if err != nil {
		return err
	}

	DefaultSigningKey = signingKey

	return nil
}

// loadSymmetricKey checks if the configured secret is valid.
// If it is not valid, it will return an error.
func loadSymmetricKey() (any, error) {
	return util.Base64FixedDecode(base64.RawURLEncoding, []byte(setting.OAuth2.JWTSecretBase64), 32)
}

// loadOrCreateAsymmetricKey checks if the configured private key exists.
// If it does not exist a new random key gets generated and saved on the configured path.
func loadOrCreateAsymmetricKey() (any, error) {
	keyPath := setting.OAuth2.JWTSigningPrivateKeyFile

	isExist, err := util.IsExist(keyPath)
	if err != nil {
		log.Fatal("Unable to check if %s exists. Error: %v", keyPath, err)
	}
	if !isExist {
		err := func() error {
			key, err := func() (any, error) {
				switch {
				case strings.HasPrefix(setting.OAuth2.JWTSigningAlgorithm, "RS"):
					return rsa.GenerateKey(rand.Reader, 4096)
				case setting.OAuth2.JWTSigningAlgorithm == "EdDSA":
					_, pk, err := ed25519.GenerateKey(rand.Reader)
					return pk, err
				default:
					return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				}
			}()
			if err != nil {
				return err
			}

			bytes, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return err
			}

			privateKeyPEM := &pem.Block{Type: "PRIVATE KEY", Bytes: bytes}

			if err := os.MkdirAll(filepath.Dir(keyPath), os.ModePerm); err != nil {
				return err
			}

			f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			defer func() {
				if err = f.Close(); err != nil {
					log.Error("Close: %v", err)
				}
			}()

			return pem.Encode(f, privateKeyPEM)
		}()
		if err != nil {
			log.Fatal("Error generating private key: %v", err)
			return nil, err
		}
	}

	bytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, fmt.Errorf("no valid PEM data found in %s", keyPath)
	} else if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("expected PRIVATE KEY, got %s in %s", block.Type, keyPath)
	}

	return x509.ParsePKCS8PrivateKey(block.Bytes)
}
