package secret

import (
	"crypto/rand"
	"encoding/base64"
)

func New() (string, error) {
	return NewWithLength(32)
}

func NewWithLength(length int64) (string, error) {
	return randomString(length)
}

func randomBytes(len int64) ([]byte, error) {
	b := make([]byte, len)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func randomString(len int64) (string, error) {
	b, err := randomBytes(len)
	return base64.URLEncoding.EncodeToString(b), err
}
