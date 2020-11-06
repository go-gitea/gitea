// @author Couchbase <info@couchbase.com>
// @copyright 2018 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package scramsha provides implementation of client side SCRAM-SHA
// according to https://tools.ietf.org/html/rfc5802
package scramsha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
	"hash"
	"strconv"
	"strings"
)

func hmacHash(message []byte, secret []byte, hashFunc func() hash.Hash) []byte {
	h := hmac.New(hashFunc, secret)
	h.Write(message)
	return h.Sum(nil)
}

func shaHash(message []byte, hashFunc func() hash.Hash) []byte {
	h := hashFunc()
	h.Write(message)
	return h.Sum(nil)
}

func generateClientNonce(size int) (string, error) {
	randomBytes := make([]byte, size)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", errors.Wrap(err, "Unable to generate nonce")
	}
	return base64.StdEncoding.EncodeToString(randomBytes), nil
}

// ScramSha provides context for SCRAM-SHA handling
type ScramSha struct {
	hashSize       int
	hashFunc       func() hash.Hash
	clientNonce    string
	serverNonce    string
	salt           []byte
	i              int
	saltedPassword []byte
	authMessage    string
}

var knownMethods = []string{"SCRAM-SHA512", "SCRAM-SHA256", "SCRAM-SHA1"}

// BestMethod returns SCRAM-SHA method we consider the best out of suggested
// by server
func BestMethod(methods string) (string, error) {
	for _, m := range knownMethods {
		if strings.Index(methods, m) != -1 {
			return m, nil
		}
	}
	return "", errors.Errorf(
		"None of the server suggested methods [%s] are supported",
		methods)
}

// NewScramSha creates context for SCRAM-SHA handling
func NewScramSha(method string) (*ScramSha, error) {
	s := &ScramSha{}

	if method == knownMethods[0] {
		s.hashFunc = sha512.New
		s.hashSize = 64
	} else if method == knownMethods[1] {
		s.hashFunc = sha256.New
		s.hashSize = 32
	} else if method == knownMethods[2] {
		s.hashFunc = sha1.New
		s.hashSize = 20
	} else {
		return nil, errors.Errorf("Unsupported method %s", method)
	}
	return s, nil
}

// GetStartRequest builds start SCRAM-SHA request to be sent to server
func (s *ScramSha) GetStartRequest(user string) (string, error) {
	var err error
	s.clientNonce, err = generateClientNonce(24)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to generate SCRAM-SHA "+
			"start request for user %s", user)
	}

	message := fmt.Sprintf("n,,n=%s,r=%s", user, s.clientNonce)
	s.authMessage = message[3:]
	return message, nil
}

// HandleStartResponse handles server response on start SCRAM-SHA request
func (s *ScramSha) HandleStartResponse(response string) error {
	parts := strings.Split(response, ",")
	if len(parts) != 3 {
		return errors.Errorf("expected 3 fields in first SCRAM-SHA-1 "+
			"server message %s", response)
	}
	if !strings.HasPrefix(parts[0], "r=") || len(parts[0]) < 3 {
		return errors.Errorf("Server sent an invalid nonce %s",
			parts[0])
	}
	if !strings.HasPrefix(parts[1], "s=") || len(parts[1]) < 3 {
		return errors.Errorf("Server sent an invalid salt %s", parts[1])
	}
	if !strings.HasPrefix(parts[2], "i=") || len(parts[2]) < 3 {
		return errors.Errorf("Server sent an invalid iteration count %s",
			parts[2])
	}

	s.serverNonce = parts[0][2:]
	encodedSalt := parts[1][2:]
	var err error
	s.i, err = strconv.Atoi(parts[2][2:])
	if err != nil {
		return errors.Errorf("Iteration count %s must be integer.",
			parts[2][2:])
	}

	if s.i < 1 {
		return errors.New("Iteration count should be positive")
	}

	if !strings.HasPrefix(s.serverNonce, s.clientNonce) {
		return errors.Errorf("Server nonce %s doesn't contain client"+
			" nonce %s", s.serverNonce, s.clientNonce)
	}

	s.salt, err = base64.StdEncoding.DecodeString(encodedSalt)
	if err != nil {
		return errors.Wrapf(err, "Unable to decode salt %s",
			encodedSalt)
	}

	s.authMessage = s.authMessage + "," + response
	return nil
}

// GetFinalRequest builds final SCRAM-SHA request to be sent to server
func (s *ScramSha) GetFinalRequest(pass string) string {
	clientFinalMessageBare := "c=biws,r=" + s.serverNonce
	s.authMessage = s.authMessage + "," + clientFinalMessageBare

	s.saltedPassword = pbkdf2.Key([]byte(pass), s.salt, s.i,
		s.hashSize, s.hashFunc)

	clientKey := hmacHash([]byte("Client Key"), s.saltedPassword, s.hashFunc)
	storedKey := shaHash(clientKey, s.hashFunc)
	clientSignature := hmacHash([]byte(s.authMessage), storedKey, s.hashFunc)

	clientProof := make([]byte, len(clientSignature))
	for i := 0; i < len(clientSignature); i++ {
		clientProof[i] = clientKey[i] ^ clientSignature[i]
	}

	return clientFinalMessageBare + ",p=" +
		base64.StdEncoding.EncodeToString(clientProof)
}

// HandleFinalResponse handles server's response on final SCRAM-SHA request
func (s *ScramSha) HandleFinalResponse(response string) error {
	if strings.Contains(response, ",") ||
		!strings.HasPrefix(response, "v=") {
		return errors.Errorf("Server sent an invalid final message %s",
			response)
	}

	decodedMessage, err := base64.StdEncoding.DecodeString(response[2:])
	if err != nil {
		return errors.Wrapf(err, "Unable to decode server message %s",
			response[2:])
	}
	serverKey := hmacHash([]byte("Server Key"), s.saltedPassword,
		s.hashFunc)
	serverSignature := hmacHash([]byte(s.authMessage), serverKey,
		s.hashFunc)
	if string(decodedMessage) != string(serverSignature) {
		return errors.Errorf("Server proof %s doesn't match "+
			"the expected: %s",
			string(decodedMessage), string(serverSignature))
	}
	return nil
}
