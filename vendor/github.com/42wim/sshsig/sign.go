// Modifications by 42wim
//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshsig

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
	"io"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L81
type MessageWrapper struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          string
}

// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L34
type WrappedSig struct {
	MagicHeader   [6]byte
	Version       uint32
	PublicKey     string
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     string
}

const (
	magicHeader          = "SSHSIG"
	defaultHashAlgorithm = "sha512"
)

var supportedHashAlgorithms = map[string]func() hash.Hash{
	"sha256": sha256.New,
	"sha512": sha512.New,
}

func wrapData(m io.Reader, ns string) ([]byte, error) {
	hf := sha512.New()
	if _, err := io.Copy(hf, m); err != nil {
		return nil, err
	}
	mh := hf.Sum(nil)

	sp := MessageWrapper{
		Namespace:     ns,
		HashAlgorithm: defaultHashAlgorithm,
		Hash:          string(mh),
	}

	dataMessageWrapper := ssh.Marshal(sp)
	dataMessageWrapper = append([]byte(magicHeader), dataMessageWrapper...)

	return dataMessageWrapper, nil
}

func sign(s ssh.AlgorithmSigner, m io.Reader, ns string) (*ssh.Signature, error) {
	dataMessageWrapper, err := wrapData(m, ns)
	if err != nil {
		return nil, err
	}
	// ssh-rsa is not supported for RSA keys:
	// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.sshsig#L71
	// We can use the default value of "" for other key types though.
	algo := ""
	if s.PublicKey().Type() == ssh.KeyAlgoRSA {
		algo = ssh.SigAlgoRSASHA2512
	}

	return s.SignWithAlgorithm(rand.Reader, dataMessageWrapper, algo)
}

func signAgent(pk ssh.PublicKey, ag agent.Agent, m io.Reader, ns string) (*ssh.Signature, error) {
	dataMessageWrapper, err := wrapData(m, ns)
	if err != nil {
		return nil, err
	}

	var sigFlag agent.SignatureFlags
	if pk.Type() == ssh.KeyAlgoRSA {
		sigFlag = agent.SignatureFlagRsaSha512
	}

	agExt, ok := ag.(agent.ExtendedAgent)
	if !ok {
		return nil, errors.New("couldn't cast to ExtendedAgent")
	}

	return agExt.SignWithFlags(pk, dataMessageWrapper, sigFlag)
}

// SignWithAgent asks the ssh Agent to sign the data with the signer matching the given publicKey and returns an armored signature.
// The purpose of the namespace value is to specify a unambiguous
// interpretation domain for the signature, e.g. file signing.
// This prevents cross-protocol attacks caused by signatures
// intended for one intended domain being accepted in another.
// If empty, the default is "file".
// This can be compared with `ssh-keygen -Y sign -f keyfile -n namespace data`
func SignWithAgent(publicKey []byte, ag agent.Agent, data io.Reader, namespace string) ([]byte, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	sig, err := signAgent(pk, ag, data, namespace)
	if err != nil {
		return nil, err
	}

	armored := Armor(sig, pk, namespace)
	return armored, nil
}

// Sign signs the data with the given private key in PEM format and returns an armored signature.
// The purpose of the namespace value is to specify a unambiguous
// interpretation domain for the signature, e.g. file signing.
// This prevents cross-protocol attacks caused by signatures
// intended for one intended domain being accepted in another.
// If empty, the default is "file".
// This can be compared with `ssh-keygen -Y sign -f keyfile -n namespace data`
func Sign(pemBytes []byte, data io.Reader, namespace string) ([]byte, error) {
	s, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}

	as, ok := s.(ssh.AlgorithmSigner)
	if !ok {
		return nil, err
	}

	if namespace == "" {
		namespace = defaultNamespace
	}

	sig, err := sign(as, data, namespace)
	if err != nil {
		return nil, err
	}

	armored := Armor(sig, s.PublicKey(), namespace)
	return armored, nil
}
