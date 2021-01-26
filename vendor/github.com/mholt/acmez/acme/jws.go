// Copyright 2020 Matthew Holt
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
//
// --- ORIGINAL LICENSE ---
//
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the THIRD-PARTY file.
//
// (This file has been modified from its original contents.)
// (And it has dragons. Don't wake the dragons.)

package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	_ "crypto/sha512" // need for EC keys
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

var errUnsupportedKey = fmt.Errorf("unknown key type; only RSA and ECDSA are supported")

// keyID is the account identity provided by a CA during registration.
type keyID string

// noKeyID indicates that jwsEncodeJSON should compute and use JWK instead of a KID.
// See jwsEncodeJSON for details.
const noKeyID = keyID("")

// // noPayload indicates jwsEncodeJSON will encode zero-length octet string
// // in a JWS request. This is called POST-as-GET in RFC 8555 and is used to make
// // authenticated GET requests via POSTing with an empty payload.
// // See https://tools.ietf.org/html/rfc8555#section-6.3 for more details.
// const noPayload = ""

// jwsEncodeEAB creates a JWS payload for External Account Binding according to RFC 8555 §7.3.4.
func jwsEncodeEAB(accountKey crypto.PublicKey, hmacKey []byte, kid keyID, url string) ([]byte, error) {
	// §7.3.4: "The 'alg' field MUST indicate a MAC-based algorithm"
	alg, sha := "HS256", crypto.SHA256

	// §7.3.4: "The 'nonce' field MUST NOT be present"
	phead, err := jwsHead(alg, "", url, kid, nil)
	if err != nil {
		return nil, err
	}

	encodedKey, err := jwkEncode(accountKey)
	if err != nil {
		return nil, err
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(encodedKey))

	payloadToSign := []byte(phead + "." + payload)

	h := hmac.New(sha256.New, hmacKey)
	h.Write(payloadToSign)
	sig := h.Sum(nil)

	return jwsFinal(sha, sig, phead, payload)
}

// jwsEncodeJSON signs claimset using provided key and a nonce.
// The result is serialized in JSON format containing either kid or jwk
// fields based on the provided keyID value.
//
// If kid is non-empty, its quoted value is inserted in the protected head
// as "kid" field value. Otherwise, JWK is computed using jwkEncode and inserted
// as "jwk" field value. The "jwk" and "kid" fields are mutually exclusive.
//
// See https://tools.ietf.org/html/rfc7515#section-7.
//
// If nonce is empty, it will not be encoded into the header.
func jwsEncodeJSON(claimset interface{}, key crypto.Signer, kid keyID, nonce, url string) ([]byte, error) {
	alg, sha := jwsHasher(key.Public())
	if alg == "" || !sha.Available() {
		return nil, errUnsupportedKey
	}

	phead, err := jwsHead(alg, nonce, url, kid, key)
	if err != nil {
		return nil, err
	}

	var payload string
	if claimset != nil {
		cs, err := json.Marshal(claimset)
		if err != nil {
			return nil, err
		}
		payload = base64.RawURLEncoding.EncodeToString(cs)
	}

	payloadToSign := []byte(phead + "." + payload)
	hash := sha.New()
	_, _ = hash.Write(payloadToSign)
	digest := hash.Sum(nil)

	sig, err := jwsSign(key, sha, digest)
	if err != nil {
		return nil, err
	}

	return jwsFinal(sha, sig, phead, payload)
}

// jwkEncode encodes public part of an RSA or ECDSA key into a JWK.
// The result is also suitable for creating a JWK thumbprint.
// https://tools.ietf.org/html/rfc7517
func jwkEncode(pub crypto.PublicKey) (string, error) {
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		// https://tools.ietf.org/html/rfc7518#section-6.3.1
		n := pub.N
		e := big.NewInt(int64(pub.E))
		// Field order is important.
		// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
		return fmt.Sprintf(`{"e":"%s","kty":"RSA","n":"%s"}`,
			base64.RawURLEncoding.EncodeToString(e.Bytes()),
			base64.RawURLEncoding.EncodeToString(n.Bytes()),
		), nil
	case *ecdsa.PublicKey:
		// https://tools.ietf.org/html/rfc7518#section-6.2.1
		p := pub.Curve.Params()
		n := p.BitSize / 8
		if p.BitSize%8 != 0 {
			n++
		}
		x := pub.X.Bytes()
		if n > len(x) {
			x = append(make([]byte, n-len(x)), x...)
		}
		y := pub.Y.Bytes()
		if n > len(y) {
			y = append(make([]byte, n-len(y)), y...)
		}
		// Field order is important.
		// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
		return fmt.Sprintf(`{"crv":"%s","kty":"EC","x":"%s","y":"%s"}`,
			p.Name,
			base64.RawURLEncoding.EncodeToString(x),
			base64.RawURLEncoding.EncodeToString(y),
		), nil
	}
	return "", errUnsupportedKey
}

// jwsHead constructs the protected JWS header for the given fields.
// Since jwk and kid are mutually-exclusive, the jwk will be encoded
// only if kid is empty. If nonce is empty, it will not be encoded.
func jwsHead(alg, nonce, url string, kid keyID, key crypto.Signer) (string, error) {
	phead := fmt.Sprintf(`{"alg":%q`, alg)
	if kid == noKeyID {
		jwk, err := jwkEncode(key.Public())
		if err != nil {
			return "", err
		}
		phead += fmt.Sprintf(`,"jwk":%s`, jwk)
	} else {
		phead += fmt.Sprintf(`,"kid":%q`, kid)
	}
	if nonce != "" {
		phead += fmt.Sprintf(`,"nonce":%q`, nonce)
	}
	phead += fmt.Sprintf(`,"url":%q}`, url)
	phead = base64.RawURLEncoding.EncodeToString([]byte(phead))
	return phead, nil
}

// jwsFinal constructs the final JWS object.
func jwsFinal(sha crypto.Hash, sig []byte, phead, payload string) ([]byte, error) {
	enc := struct {
		Protected string `json:"protected"`
		Payload   string `json:"payload"`
		Sig       string `json:"signature"`
	}{
		Protected: phead,
		Payload:   payload,
		Sig:       base64.RawURLEncoding.EncodeToString(sig),
	}
	result, err := json.Marshal(&enc)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// jwsSign signs the digest using the given key.
// The hash is unused for ECDSA keys.
//
// Note: non-stdlib crypto.Signer implementations are expected to return
// the signature in the format as specified in RFC7518.
// See https://tools.ietf.org/html/rfc7518 for more details.
func jwsSign(key crypto.Signer, hash crypto.Hash, digest []byte) ([]byte, error) {
	if key, ok := key.(*ecdsa.PrivateKey); ok {
		// The key.Sign method of ecdsa returns ASN1-encoded signature.
		// So, we use the package Sign function instead
		// to get R and S values directly and format the result accordingly.
		r, s, err := ecdsa.Sign(rand.Reader, key, digest)
		if err != nil {
			return nil, err
		}
		rb, sb := r.Bytes(), s.Bytes()
		size := key.Params().BitSize / 8
		if size%8 > 0 {
			size++
		}
		sig := make([]byte, size*2)
		copy(sig[size-len(rb):], rb)
		copy(sig[size*2-len(sb):], sb)
		return sig, nil
	}
	return key.Sign(rand.Reader, digest, hash)
}

// jwsHasher indicates suitable JWS algorithm name and a hash function
// to use for signing a digest with the provided key.
// It returns ("", 0) if the key is not supported.
func jwsHasher(pub crypto.PublicKey) (string, crypto.Hash) {
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		return "RS256", crypto.SHA256
	case *ecdsa.PublicKey:
		switch pub.Params().Name {
		case "P-256":
			return "ES256", crypto.SHA256
		case "P-384":
			return "ES384", crypto.SHA384
		case "P-521":
			return "ES512", crypto.SHA512
		}
	}
	return "", 0
}

// jwkThumbprint creates a JWK thumbprint out of pub
// as specified in https://tools.ietf.org/html/rfc7638.
func jwkThumbprint(pub crypto.PublicKey) (string, error) {
	jwk, err := jwkEncode(pub)
	if err != nil {
		return "", err
	}
	b := sha256.Sum256([]byte(jwk))
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
