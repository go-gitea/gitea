package application

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	jwt "github.com/golang-jwt/jwt/v5"
)

type ErrInvalidPublicKey struct {
	Key string
}

func (e ErrInvalidPublicKey) Error() string {
	return "invalid public key: " + e.Key
}

func (e ErrInvalidPublicKey) Unwrap() error {
	return util.ErrInvalidArgument
}

func (ext *AppExternalData) AddJWTPublicKey(ctx context.Context, key string) error {
	block, _ := pem.Decode([]byte(key))
	if block == nil || block.Type != "PUBLIC KEY" {
		return ErrInvalidPublicKey{Key: key}
	}
	_, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return ErrInvalidPublicKey{Key: key}
	}

	hash := sha256.Sum256([]byte(key))
	sha256Str := base64.StdEncoding.EncodeToString(hash[:])

	for _, k := range ext.JWTKeyList {
		if k.RawKeySHA == sha256Str {
			// Key already exists, do nothing
			return nil
		}
	}

	ext.JWTKeyList = append(ext.JWTKeyList, JWTPubKey{
		RawKey:    key,
		RawKeySHA: sha256Str,
	})

	_, err = db.GetEngine(ctx).ID(ext.ID).
		Cols("jwt_key_list").Update(ext)
	return err
}

func (ext *AppExternalData) DeleteJWTPublicKey(ctx context.Context, keySHA string) error {
	newKeyList := make([]JWTPubKey, 0, len(ext.JWTKeyList))

	for _, k := range ext.JWTKeyList {
		if k.RawKeySHA != keySHA {
			newKeyList = append(newKeyList, k)
		}
	}

	if len(newKeyList) == len(ext.JWTKeyList) {
		// Key not found, do nothing
		return nil
	}

	ext.JWTKeyList = newKeyList

	_, err := db.GetEngine(ctx).ID(ext.ID).
		Cols("jwt_key_list").Update(ext)
	return err
}

func (a *Application) AddJWTPublicKey(ctx context.Context, key string) error {

	if err := a.LoadExternalData(ctx); err != nil {
		return err
	}

	return a.AppExternalData().AddJWTPublicKey(ctx, key)
}

type JWTClaims struct {
	App   *Application `json:"-"`
	Scope string       `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

func GetAppByClientID(ctx context.Context, clientID string) (*Application, error) {
	type joinedApp struct {
		App          *Application     `xorm:"extends"`
		ExternalData *AppExternalData `xorm:"extends"`
	}

	app := new(joinedApp)

	if ok, err := db.GetEngine(ctx).
		Table("user").
		Where("client_id = ?", clientID).
		Join("INNER", "app_external_data", "uid = user.id").
		Get(app); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrAppNotExist{ClientID: clientID}
	}

	app.App.ExternalData = app.ExternalData

	return app.App, nil
}

func (k JWTPubKey) LoadRSAPublicKey() *rsa.PublicKey {
	block, _ := pem.Decode([]byte(k.RawKey))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil
	}

	return rsaPub
}

func (app *Application) GetJWTVerificationKeyList() []jwt.VerificationKey {
	verificationKeys := make([]jwt.VerificationKey, 0, len(app.AppExternalData().JWTKeyList))

	for _, k := range app.AppExternalData().JWTKeyList {
		rsaKey := k.LoadRSAPublicKey()
		if rsaKey == nil {
			continue
		}

		verificationKeys = append(verificationKeys, rsaKey)
	}

	return verificationKeys
}

type ErrInvalidJWTSignature struct {
	Message string
}

func (e ErrInvalidJWTSignature) Error() string {
	return "invalid JWT signature: " + e.Message
}

func (e ErrInvalidJWTSignature) Unwrap() error {
	return util.ErrInvalidArgument
}

func ValidateJWTSignature(ctx context.Context, tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		claims, ok := token.Claims.(*JWTClaims)
		if !ok {
			return nil, errors.New("invalid token claims structure")
		}

		clientID := claims.Issuer
		if clientID == "" {
			return nil, errors.New("missing issuer (client_id) in token")
		}

		app, err := GetAppByClientID(ctx, clientID)
		if err != nil {
			return nil, fmt.Errorf("failed to get app by client_id: %v", err)
		}

		claims.App = app

		return jwt.VerificationKeySet{
			Keys: app.GetJWTVerificationKeyList(),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return token.Claims.(*JWTClaims), nil
}

type JWTkeyPair struct {
	PrivateKey *rsa.PrivateKey
}

func (j *JWTkeyPair) PublicKeyPEM() (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(&j.PrivateKey.PublicKey)
	if err != nil {
		return "", err
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}

	writer := new(bytes.Buffer)
	if err := pem.Encode(writer, pubBlock); err != nil {
		return "", err
	}

	return writer.String(), nil
}

func (j *JWTkeyPair) PrivateKeyPEM() (string, error) {
	privBytes := x509.MarshalPKCS1PrivateKey(j.PrivateKey)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}

	writer := new(bytes.Buffer)
	if err := pem.Encode(writer, privBlock); err != nil {
		return "", err
	}

	return writer.String(), nil
}

const DefaultJWTkeySize = 2048

func GenerateKeyPair() (*JWTkeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, DefaultJWTkeySize)
	if err != nil {
		return nil, err
	}

	return &JWTkeyPair{PrivateKey: key}, nil
}

func CreateJWTToken(key *rsa.PrivateKey, clientID string, timeout int64) (string, error) {
	claims := jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Unix() + timeout,
		"iss": clientID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	jwtStr, err := token.SignedString(key)
	if err != nil {
		fmt.Printf("Failed to sign JWT: %v\n", err)
		return "", err
	}

	return jwtStr, nil
}
