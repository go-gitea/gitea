// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/dgrijalva/jwt-go"
	ini "gopkg.in/ini.v1"
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

	return map[string]string{
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

	return map[string]string{
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
func InitSigningKey() (err error) {
	var key interface{}

	switch setting.OAuth2.JWTSigningAlgorithm {
	case "HS256":
		fallthrough
	case "HS384":
		fallthrough
	case "HS512":
		key, err = loadOrCreateSymmetricKey()

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
		key, err = loadOrCreateAsymmetricKey()

	default:
		return ErrInvalidAlgorithmType{setting.OAuth2.JWTSigningAlgorithm}
	}

	if err != nil {
		log.Error("Error while loading or creating symmetric key: %v", err)
		return
	}

	signingKey, err := CreateJWTSingingKey(setting.OAuth2.JWTSigningAlgorithm, key)
	if err != nil {
		return
	}

	DefaultSigningKey = signingKey

	return
}

func loadOrCreateSymmetricKey() (interface{}, error) {
	key := make([]byte, 32)
	n, err := base64.RawURLEncoding.Decode(key, []byte(setting.OAuth2.JWTSecretBase64))
	if err != nil || n != 32 {
		key, err = generate.NewJwtSecret()
		if err != nil {
			log.Fatal("error generating JWT secret: %v", err)
			return nil, err
		}

		setting.CreateOrAppendToCustomConf(func(cfg *ini.File) {
			secretBase64 := base64.RawURLEncoding.EncodeToString(key)
			cfg.Section("oauth2").Key("JWT_SECRET").SetValue(secretBase64)
		})
	}

	return key, nil
}

func loadOrCreateAsymmetricKey() (interface{}, error) {
	keyPath := setting.OAuth2.JWTSigningPrivateKeyFile

	isExist, err := util.IsExist(keyPath)
	if err != nil {
		log.Fatal("Unable to check if %s exists. Error: %v", keyPath, err)
	}
	if !isExist {
		err := func() error {
			key, err := func() (interface{}, error) {
				if strings.HasPrefix(setting.OAuth2.JWTSigningAlgorithm, "RS") {
					return rsa.GenerateKey(rand.Reader, 4096)
				}
				return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

			f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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

	bytes, err := ioutil.ReadFile(keyPath)
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
