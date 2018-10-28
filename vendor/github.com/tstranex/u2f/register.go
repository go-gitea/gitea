// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"time"
)

// Registration represents a single enrolment or pairing between an
// application and a token. This data will typically be stored in a database.
type Registration struct {
	// Raw serialized registration data as received from the token.
	Raw []byte

	KeyHandle []byte
	PubKey    ecdsa.PublicKey

	// AttestationCert can be nil for Authenticate requests.
	AttestationCert *x509.Certificate
}

// Config contains configurable options for the package.
type Config struct {
	// SkipAttestationVerify controls whether the token attestation
	// certificate should be verified on registration. Ideally it should
	// always be verified. However, there is currently no public list of
	// trusted attestation root certificates so it may be necessary to skip.
	SkipAttestationVerify bool

	// RootAttestationCertPool overrides the default root certificates used
	// to verify client attestations. If nil, this defaults to the roots that are
	// bundled in this library.
	RootAttestationCertPool *x509.CertPool
}

// Register validates a RegisterResponse message to enrol a new token.
// An error is returned if any part of the response fails to validate.
// The returned Registration should be stored by the caller.
func Register(resp RegisterResponse, c Challenge, config *Config) (*Registration, error) {
	if config == nil {
		config = &Config{}
	}

	if time.Now().Sub(c.Timestamp) > timeout {
		return nil, errors.New("u2f: challenge has expired")
	}

	regData, err := decodeBase64(resp.RegistrationData)
	if err != nil {
		return nil, err
	}

	clientData, err := decodeBase64(resp.ClientData)
	if err != nil {
		return nil, err
	}

	reg, sig, err := parseRegistration(regData)
	if err != nil {
		return nil, err
	}

	if err := verifyClientData(clientData, c); err != nil {
		return nil, err
	}

	if err := verifyAttestationCert(*reg, config); err != nil {
		return nil, err
	}

	if err := verifyRegistrationSignature(*reg, sig, c.AppID, clientData); err != nil {
		return nil, err
	}

	return reg, nil
}

func parseRegistration(buf []byte) (*Registration, []byte, error) {
	if len(buf) < 1+65+1+1+1 {
		return nil, nil, errors.New("u2f: data is too short")
	}

	var r Registration
	r.Raw = buf

	if buf[0] != 0x05 {
		return nil, nil, errors.New("u2f: invalid reserved byte")
	}
	buf = buf[1:]

	x, y := elliptic.Unmarshal(elliptic.P256(), buf[:65])
	if x == nil {
		return nil, nil, errors.New("u2f: invalid public key")
	}
	r.PubKey.Curve = elliptic.P256()
	r.PubKey.X = x
	r.PubKey.Y = y
	buf = buf[65:]

	khLen := int(buf[0])
	buf = buf[1:]
	if len(buf) < khLen {
		return nil, nil, errors.New("u2f: invalid key handle")
	}
	r.KeyHandle = buf[:khLen]
	buf = buf[khLen:]

	// The length of the x509 cert isn't specified so it has to be inferred
	// by parsing. We can't use x509.ParseCertificate yet because it returns
	// an error if there are any trailing bytes. So parse raw asn1 as a
	// workaround to get the length.
	sig, err := asn1.Unmarshal(buf, &asn1.RawValue{})
	if err != nil {
		return nil, nil, err
	}

	buf = buf[:len(buf)-len(sig)]
	fixCertIfNeed(buf)
	cert, err := x509.ParseCertificate(buf)
	if err != nil {
		return nil, nil, err
	}
	r.AttestationCert = cert

	return &r, sig, nil
}

// UnmarshalBinary implements encoding.BinaryMarshaler.
func (r *Registration) UnmarshalBinary(data []byte) error {
	reg, _, err := parseRegistration(data)
	if err != nil {
		return err
	}
	*r = *reg
	return nil
}

// MarshalBinary implements encoding.BinaryUnmarshaler.
func (r *Registration) MarshalBinary() ([]byte, error) {
	return r.Raw, nil
}

func verifyAttestationCert(r Registration, config *Config) error {
	if config.SkipAttestationVerify {
		return nil
	}
	rootCertPool := roots
	if config.RootAttestationCertPool != nil {
		rootCertPool = config.RootAttestationCertPool
	}

	opts := x509.VerifyOptions{Roots: rootCertPool}
	_, err := r.AttestationCert.Verify(opts)
	return err
}

func verifyRegistrationSignature(
	r Registration, signature []byte, appid string, clientData []byte) error {

	appParam := sha256.Sum256([]byte(appid))
	challenge := sha256.Sum256(clientData)

	buf := []byte{0}
	buf = append(buf, appParam[:]...)
	buf = append(buf, challenge[:]...)
	buf = append(buf, r.KeyHandle...)
	pk := elliptic.Marshal(r.PubKey.Curve, r.PubKey.X, r.PubKey.Y)
	buf = append(buf, pk...)

	return r.AttestationCert.CheckSignature(
		x509.ECDSAWithSHA256, buf, signature)
}

func getRegisteredKey(appID string, r Registration) RegisteredKey {
	return RegisteredKey{
		Version:   u2fVersion,
		KeyHandle: encodeBase64(r.KeyHandle),
		AppID:     appID,
	}
}

// fixCertIfNeed fixes broken certificates described in
// https://github.com/Yubico/php-u2flib-server/blob/master/src/u2flib_server/U2F.php#L84
func fixCertIfNeed(cert []byte) {
	h := sha256.Sum256(cert)
	switch hex.EncodeToString(h[:]) {
	case
		"349bca1031f8c82c4ceca38b9cebf1a69df9fb3b94eed99eb3fb9aa3822d26e8",
		"dd574527df608e47ae45fbba75a2afdd5c20fd94a02419381813cd55a2a3398f",
		"1d8764f0f7cd1352df6150045c8f638e517270e8b5dda1c63ade9c2280240cae",
		"d0edc9a91a1677435a953390865d208c55b3183c6759c9b5a7ff494c322558eb",
		"6073c436dcd064a48127ddbf6032ac1a66fd59a0c24434f070d4e564c124c897",
		"ca993121846c464d666096d35f13bf44c1b05af205f9b4a1e00cf6cc10c5e511":

		// clear the offending byte.
		cert[len(cert)-257] = 0
	}
}

// NewWebRegisterRequest creates a request to enrol a new token.
// regs is the list of the user's existing registration. The browser will
// refuse to re-register a device if it has an existing registration.
func NewWebRegisterRequest(c *Challenge, regs []Registration) *WebRegisterRequest {
	req := RegisterRequest{
		Version:   u2fVersion,
		Challenge: encodeBase64(c.Challenge),
	}

	rr := WebRegisterRequest{
		AppID:            c.AppID,
		RegisterRequests: []RegisterRequest{req},
	}

	for _, r := range regs {
		rk := getRegisteredKey(c.AppID, r)
		rr.RegisteredKeys = append(rr.RegisteredKeys, rk)
	}

	return &rr
}
