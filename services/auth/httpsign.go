// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/httpsig"
	"golang.org/x/crypto/ssh"
)

// Ensure the struct implements the interface.
var (
	_ Method = &HTTPSign{}
)

// HTTPSign implements the Auth interface and authenticates requests (API requests
// only) by looking for http signature data in the "Signature" header.
// more information can be found on https://github.com/go-fed/httpsig
type HTTPSign struct{}

// Name represents the name of auth method
func (h *HTTPSign) Name() string {
	return "httpsign"
}

// Verify extracts and validates HTTPsign from the Signature header of the request and returns
// the corresponding user object on successful validation.
// Returns nil if header is empty or validation fails.
func (h *HTTPSign) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	sigHead := req.Header.Get("Signature")
	if len(sigHead) == 0 {
		return nil, nil
	}

	var (
		publicKey *asymkey_model.PublicKey
		err       error
	)

	if len(req.Header.Get("X-Ssh-Certificate")) != 0 {
		// Handle Signature signed by SSH certificates
		if len(setting.SSH.TrustedUserCAKeys) == 0 {
			return nil, nil
		}

		publicKey, err = VerifyCert(req)
		if err != nil {
			log.Debug("VerifyCert on request from %s: failed: %v", req.RemoteAddr, err)
			log.Warn("Failed authentication attempt from %s", req.RemoteAddr)
			return nil, nil
		}
	} else {
		// Handle Signature signed by Public Key
		publicKey, err = VerifyPubKey(req)
		if err != nil {
			log.Debug("VerifyPubKey on request from %s: failed: %v", req.RemoteAddr, err)
			log.Warn("Failed authentication attempt from %s", req.RemoteAddr)
			return nil, nil
		}
	}

	u, err := user_model.GetUserByID(req.Context(), publicKey.OwnerID)
	if err != nil {
		log.Error("GetUserByID:  %v", err)
		return nil, err
	}

	store.GetData()["IsApiToken"] = true

	log.Trace("HTTP Sign: Logged in user %-v", u)

	return u, nil
}

func VerifyPubKey(r *http.Request) (*asymkey_model.PublicKey, error) {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return nil, fmt.Errorf("httpsig.NewVerifier failed: %s", err)
	}

	keyID := verifier.KeyId()

	publicKeys, err := asymkey_model.SearchPublicKey(r.Context(), 0, keyID)
	if err != nil {
		return nil, err
	}

	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("no public key found for keyid %s", keyID)
	}

	sshPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeys[0].Content))
	if err != nil {
		return nil, err
	}

	if err := doVerify(verifier, []ssh.PublicKey{sshPublicKey}); err != nil {
		return nil, err
	}

	return publicKeys[0], nil
}

// VerifyCert verifies the validity of the ssh certificate and returns the publickey of the signer
// We verify that the certificate is signed with the correct CA
// We verify that the http request is signed with the private key (of the public key mentioned in the certificate)
func VerifyCert(r *http.Request) (*asymkey_model.PublicKey, error) {
	// Get our certificate from the header
	bcert, err := base64.RawStdEncoding.DecodeString(r.Header.Get("x-ssh-certificate"))
	if err != nil {
		return nil, err
	}

	pk, err := ssh.ParsePublicKey(bcert)
	if err != nil {
		return nil, err
	}

	// Check if it's really a ssh certificate
	cert, ok := pk.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("no certificate found")
	}

	c := &ssh.CertChecker{
		IsUserAuthority: func(auth ssh.PublicKey) bool {
			marshaled := auth.Marshal()

			for _, k := range setting.SSH.TrustedUserCAKeysParsed {
				if bytes.Equal(marshaled, k.Marshal()) {
					return true
				}
			}

			return false
		},
	}

	// check the CA of the cert
	if !c.IsUserAuthority(cert.SignatureKey) {
		return nil, fmt.Errorf("CA check failed")
	}

	// Create a verifier
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return nil, fmt.Errorf("httpsig.NewVerifier failed: %s", err)
	}

	// now verify that this request was signed with the private key that matches the certificate public key
	if err := doVerify(verifier, []ssh.PublicKey{cert.Key}); err != nil {
		return nil, err
	}

	// Now for each of the certificate valid principals
	for _, principal := range cert.ValidPrincipals {
		// Look in the db for the public key
		publicKey, err := asymkey_model.SearchPublicKeyByContentExact(r.Context(), principal)
		if asymkey_model.IsErrKeyNotExist(err) {
			// No public key matches this principal - try the next principal
			continue
		} else if err != nil {
			// this error will be a db error therefore we can't solve this and we should abort
			log.Error("SearchPublicKeyByContentExact: %v", err)
			return nil, err
		}

		// Validate the cert for this principal
		if err := c.CheckCert(principal, cert); err != nil {
			// however, because principal is a member of ValidPrincipals - if this fails then the certificate itself is invalid
			return nil, err
		}

		// OK we have a public key for a principal matching a valid certificate whose key has signed this request.
		return publicKey, nil
	}

	// No public key matching a principal in the certificate is registered in gitea
	return nil, fmt.Errorf("no valid principal found")
}

// doVerify iterates across the provided public keys attempting the verify the current request against each key in turn
func doVerify(verifier httpsig.Verifier, sshPublicKeys []ssh.PublicKey) error {
	for _, publicKey := range sshPublicKeys {
		cryptoPubkey := publicKey.(ssh.CryptoPublicKey).CryptoPublicKey()

		var algos []httpsig.Algorithm

		switch {
		case strings.HasPrefix(publicKey.Type(), "ssh-ed25519"):
			algos = []httpsig.Algorithm{httpsig.ED25519}
		case strings.HasPrefix(publicKey.Type(), "ssh-rsa"):
			algos = []httpsig.Algorithm{httpsig.RSA_SHA1, httpsig.RSA_SHA256, httpsig.RSA_SHA512}
		}
		for _, algo := range algos {
			if err := verifier.Verify(cryptoPubkey, algo); err == nil {
				return nil
			}
		}
	}

	return errors.New("verification failed")
}
