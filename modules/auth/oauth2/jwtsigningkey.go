// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"

	"code.gitea.io/gitea/modules/setting"

	"github.com/dgrijalva/jwt-go"
)

// ErrInvalidAlgorithmType represents an invalid algorithm error.
type ErrInvalidAlgorithmType struct {
	Algorightm string
}

func (err ErrInvalidAlgorithmType) Error() string {
	return fmt.Sprintf("JWT signing algorithm is not supported: %s", err.Algorightm)
}

// JWTSigningKey represents a algorithm/key pair to sign JWTs
type JWTSigningKey interface {
	IsSymmetric() bool
	SigningMethod() jwt.SigningMethod
	SignKey() interface{}
	VerifyKey() interface{}
	ToJSON() map[string]string
}

type hmacSingingKey struct {
	signingMethod jwt.SigningMethod
	secret        []byte
}

func (key hmacSingingKey) IsSymmetric() bool {
	return true
}

func (key hmacSingingKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key hmacSingingKey) SignKey() interface{} {
	return key.secret
}

func (key hmacSingingKey) VerifyKey() interface{} {
	return key.secret
}

func (key hmacSingingKey) ToJSON() map[string]string {
	return map[string]string{}
}

type rsaSingingKey struct {
	signingMethod jwt.SigningMethod
	key           *rsa.PrivateKey
}

func (key rsaSingingKey) IsSymmetric() bool {
	return false
}

func (key rsaSingingKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key rsaSingingKey) SignKey() interface{} {
	return key.key
}

func (key rsaSingingKey) VerifyKey() interface{} {
	return key.key.Public()
}

func (key rsaSingingKey) ToJSON() map[string]string {
	pubKey := key.key.Public().(*rsa.PublicKey)

	return map[string]string {
		"kty": "RSA",
		"alg": key.SigningMethod().Alg(),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
		"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
	}
}

type ecdsaSingingKey struct {
	signingMethod jwt.SigningMethod
	key           *ecdsa.PrivateKey
}

func (key ecdsaSingingKey) IsSymmetric() bool {
	return false
}

func (key ecdsaSingingKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key ecdsaSingingKey) SignKey() interface{} {
	return key.key
}

func (key ecdsaSingingKey) VerifyKey() interface{} {
	return key.key.Public()
}

func (key ecdsaSingingKey) ToJSON() map[string]string {
	pubKey := key.key.Public().(*ecdsa.PublicKey)

	return map[string]string {
		"kty": "EC",
		"alg": key.SigningMethod().Alg(),
		"crv": pubKey.Params().Name,
		"x":   base64.RawURLEncoding.EncodeToString(pubKey.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(pubKey.Y.Bytes()),
	}
}

// CreateJWTSingingKey creates a signing key from an algorithm / key pair.
func CreateJWTSingingKey(algorithm string, key interface{}) (JWTSigningKey, error) {
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
	default:
		return nil, ErrInvalidAlgorithmType{algorithm}
	}

	switch signingMethod.(type) {
	case *jwt.SigningMethodECDSA:
		privateKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return ecdsaSingingKey{signingMethod, privateKey}, nil
	case *jwt.SigningMethodRSA:
		privateKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return rsaSingingKey{signingMethod, privateKey}, nil
	default:
		secret, ok := key.([]byte)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return hmacSingingKey{signingMethod, secret}, nil
	}
}

// DefaultSigningKey is the default signing key for JWTs.
var DefaultSigningKey JWTSigningKey

// InitSigningKey creates the default signing key from settings or creates a random key.
func InitSigningKey() error {
	key, err := CreateJWTSingingKey("HS256", setting.OAuth2.JWTSecretBytes)
	if err != nil {
		return err
	}

	DefaultSigningKey = key

	return nil
}
