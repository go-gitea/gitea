// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"math/big"
	"time"
)

// SignRequest creates a request to initiate an authentication.
func (c *Challenge) SignRequest(regs []Registration) *WebSignRequest {
	var sr WebSignRequest
	sr.AppID = c.AppID
	sr.Challenge = encodeBase64(c.Challenge)
	for _, r := range regs {
		rk := getRegisteredKey(c.AppID, r)
		sr.RegisteredKeys = append(sr.RegisteredKeys, rk)
	}
	return &sr
}

// ErrCounterTooLow is raised when the counter value received from the device is
// lower than last stored counter value. This may indicate that the device has
// been cloned (or is malfunctioning). The application may choose to disable
// the particular device as precaution.
var ErrCounterTooLow = errors.New("u2f: counter too low")

// Authenticate validates a SignResponse authentication response.
// An error is returned if any part of the response fails to validate.
// The counter should be the counter associated with appropriate device
// (i.e. resp.KeyHandle).
// The latest counter value is returned, which the caller should store.
func (reg *Registration) Authenticate(resp SignResponse, c Challenge, counter uint32) (newCounter uint32, err error) {
	if time.Now().Sub(c.Timestamp) > timeout {
		return 0, errors.New("u2f: challenge has expired")
	}
	if resp.KeyHandle != encodeBase64(reg.KeyHandle) {
		return 0, errors.New("u2f: wrong key handle")
	}

	sigData, err := decodeBase64(resp.SignatureData)
	if err != nil {
		return 0, err
	}

	clientData, err := decodeBase64(resp.ClientData)
	if err != nil {
		return 0, err
	}

	ar, err := parseSignResponse(sigData)
	if err != nil {
		return 0, err
	}

	if ar.Counter < counter {
		return 0, ErrCounterTooLow
	}

	if err := verifyClientData(clientData, c); err != nil {
		return 0, err
	}

	if err := verifyAuthSignature(*ar, &reg.PubKey, c.AppID, clientData); err != nil {
		return 0, err
	}

	if !ar.UserPresenceVerified {
		return 0, errors.New("u2f: user was not present")
	}

	return ar.Counter, nil
}

type ecdsaSig struct {
	R, S *big.Int
}

type authResp struct {
	UserPresenceVerified bool
	Counter              uint32
	sig                  ecdsaSig
	raw                  []byte
}

func parseSignResponse(sd []byte) (*authResp, error) {
	if len(sd) < 5 {
		return nil, errors.New("u2f: data is too short")
	}

	var ar authResp

	userPresence := sd[0]
	if userPresence|1 != 1 {
		return nil, errors.New("u2f: invalid user presence byte")
	}
	ar.UserPresenceVerified = userPresence == 1

	ar.Counter = uint32(sd[1])<<24 | uint32(sd[2])<<16 | uint32(sd[3])<<8 | uint32(sd[4])

	ar.raw = sd[:5]

	rest, err := asn1.Unmarshal(sd[5:], &ar.sig)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, errors.New("u2f: trailing data")
	}

	return &ar, nil
}

func verifyAuthSignature(ar authResp, pubKey *ecdsa.PublicKey, appID string, clientData []byte) error {
	appParam := sha256.Sum256([]byte(appID))
	challenge := sha256.Sum256(clientData)

	var buf []byte
	buf = append(buf, appParam[:]...)
	buf = append(buf, ar.raw...)
	buf = append(buf, challenge[:]...)
	hash := sha256.Sum256(buf)

	if !ecdsa.Verify(pubKey, hash[:], ar.sig.R, ar.sig.S) {
		return errors.New("u2f: invalid signature")
	}

	return nil
}
