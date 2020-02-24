// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openpgp

import (
	"crypto/hmac"
	"encoding/binary"
	"io"
	"time"

	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/keybase/go-crypto/openpgp/errors"
	"github.com/keybase/go-crypto/openpgp/packet"
	"github.com/keybase/go-crypto/rsa"
)

// PublicKeyType is the armor type for a PGP public key.
var PublicKeyType = "PGP PUBLIC KEY BLOCK"

// PrivateKeyType is the armor type for a PGP private key.
var PrivateKeyType = "PGP PRIVATE KEY BLOCK"

// An Entity represents the components of an OpenPGP key: a primary public key
// (which must be a signing key), one or more identities claimed by that key,
// and zero or more subkeys, which may be encryption keys.
type Entity struct {
	PrimaryKey  *packet.PublicKey
	PrivateKey  *packet.PrivateKey
	Identities  map[string]*Identity // indexed by Identity.Name
	Revocations []*packet.Signature
	// Revocations that are signed by designated revokers. Reading keys
	// will not verify these revocations, because it won't have access to
	// issuers' public keys, API consumers should do this instead (or
	// not, and just assume that the key is probably revoked).
	UnverifiedRevocations []*packet.Signature
	Subkeys               []Subkey
	BadSubkeys            []BadSubkey
}

// An Identity represents an identity claimed by an Entity and zero or more
// assertions by other entities about that claim.
type Identity struct {
	Name          string // by convention, has the form "Full Name (comment) <email@example.com>"
	UserId        *packet.UserId
	SelfSignature *packet.Signature
	Signatures    []*packet.Signature
	Revocation    *packet.Signature
}

// A Subkey is an additional public key in an Entity. Subkeys can be used for
// encryption.
type Subkey struct {
	PublicKey  *packet.PublicKey
	PrivateKey *packet.PrivateKey
	Sig        *packet.Signature
	Revocation *packet.Signature
}

// BadSubkey is one that failed reconstruction, but we'll keep it around for
// informational purposes.
type BadSubkey struct {
	Subkey
	Err error
}

// A Key identifies a specific public key in an Entity. This is either the
// Entity's primary key or a subkey.
type Key struct {
	Entity        *Entity
	PublicKey     *packet.PublicKey
	PrivateKey    *packet.PrivateKey
	SelfSignature *packet.Signature
	KeyFlags      packet.KeyFlagBits
}

// A KeyRing provides access to public and private keys.
type KeyRing interface {

	// KeysById returns the set of keys that have the given key id.
	// fp can be optionally supplied, which is the full key fingerprint.
	// If it's provided, then it must match. This comes up in the case
	// of GPG subpacket 33.
	KeysById(id uint64, fp []byte) []Key

	// KeysByIdAndUsage returns the set of keys with the given id
	// that also meet the key usage given by requiredUsage.
	// The requiredUsage is expressed as the bitwise-OR of
	// packet.KeyFlag* values.
	// fp can be optionally supplied, which is the full key fingerprint.
	// If it's provided, then it must match. This comes up in the case
	// of GPG subpacket 33.
	KeysByIdUsage(id uint64, fp []byte, requiredUsage byte) []Key

	// DecryptionKeys returns all private keys that are valid for
	// decryption.
	DecryptionKeys() []Key
}

// primaryIdentity returns the Identity marked as primary or the first identity
// if none are so marked.
func (e *Entity) primaryIdentity() *Identity {
	var firstIdentity *Identity
	for _, ident := range e.Identities {
		if firstIdentity == nil {
			firstIdentity = ident
		}
		if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			return ident
		}
	}
	return firstIdentity
}

// encryptionKey returns the best candidate Key for encrypting a message to the
// given Entity.
func (e *Entity) encryptionKey(now time.Time) (Key, bool) {
	candidateSubkey := -1

	// Iterate the keys to find the newest, non-revoked key that can
	// encrypt.
	var maxTime time.Time
	for i, subkey := range e.Subkeys {

		// NOTE(maxtaco)
		// If there is a Flags subpacket, then we have to follow it, and only
		// use keys that are marked for Encryption of Communication.  If there
		// isn't a Flags subpacket, and this is an Encrypt-Only key (right now only ElGamal
		// suffices), then we implicitly use it. The check for primary below is a little
		// more open-ended, but for now, let's be strict and potentially open up
		// if we see bugs in the wild.
		//
		// One more note: old DSA/ElGamal keys tend not to have the Flags subpacket,
		// so this sort of thing is pretty important for encrypting to older keys.
		//
		if ((subkey.Sig.FlagsValid && subkey.Sig.FlagEncryptCommunications) ||
			(!subkey.Sig.FlagsValid && subkey.PublicKey.PubKeyAlgo == packet.PubKeyAlgoElGamal)) &&
			subkey.PublicKey.PubKeyAlgo.CanEncrypt() &&
			!subkey.Sig.KeyExpired(now) &&
			subkey.Revocation == nil &&
			(maxTime.IsZero() || subkey.Sig.CreationTime.After(maxTime)) {
			candidateSubkey = i
			maxTime = subkey.Sig.CreationTime
		}
	}

	if candidateSubkey != -1 {
		subkey := e.Subkeys[candidateSubkey]
		return Key{e, subkey.PublicKey, subkey.PrivateKey, subkey.Sig, subkey.Sig.GetKeyFlags()}, true
	}

	// If we don't have any candidate subkeys for encryption and
	// the primary key doesn't have any usage metadata then we
	// assume that the primary key is ok. Or, if the primary key is
	// marked as ok to encrypt to, then we can obviously use it.
	//
	// NOTE(maxtaco) - see note above, how this policy is a little too open-ended
	// for my liking, but leave it for now.
	i := e.primaryIdentity()
	if (!i.SelfSignature.FlagsValid || i.SelfSignature.FlagEncryptCommunications) &&
		e.PrimaryKey.PubKeyAlgo.CanEncrypt() &&
		!i.SelfSignature.KeyExpired(now) {
		return Key{e, e.PrimaryKey, e.PrivateKey, i.SelfSignature, i.SelfSignature.GetKeyFlags()}, true
	}

	// This Entity appears to be signing only.
	return Key{}, false
}

// signingKey return the best candidate Key for signing a message with this
// Entity.
func (e *Entity) signingKey(now time.Time) (Key, bool) {
	candidateSubkey := -1

	// Iterate the keys to find the newest, non-revoked key that can
	// sign.
	var maxTime time.Time
	for i, subkey := range e.Subkeys {
		if (!subkey.Sig.FlagsValid || subkey.Sig.FlagSign) &&
			subkey.PrivateKey.PrivateKey != nil &&
			subkey.PublicKey.PubKeyAlgo.CanSign() &&
			!subkey.Sig.KeyExpired(now) &&
			subkey.Revocation == nil &&
			(maxTime.IsZero() || subkey.Sig.CreationTime.After(maxTime)) {
			candidateSubkey = i
			maxTime = subkey.Sig.CreationTime
			break
		}
	}

	if candidateSubkey != -1 {
		subkey := e.Subkeys[candidateSubkey]
		return Key{e, subkey.PublicKey, subkey.PrivateKey, subkey.Sig, subkey.Sig.GetKeyFlags()}, true
	}

	// If we have no candidate subkey then we assume that it's ok to sign
	// with the primary key.
	i := e.primaryIdentity()
	if (!i.SelfSignature.FlagsValid || i.SelfSignature.FlagSign) &&
		e.PrimaryKey.PubKeyAlgo.CanSign() &&
		!i.SelfSignature.KeyExpired(now) &&
		e.PrivateKey.PrivateKey != nil {
		return Key{e, e.PrimaryKey, e.PrivateKey, i.SelfSignature, i.SelfSignature.GetKeyFlags()}, true
	}

	return Key{}, false
}

// An EntityList contains one or more Entities.
type EntityList []*Entity

func keyMatchesIdAndFingerprint(key *packet.PublicKey, id uint64, fp []byte) bool {
	if key.KeyId != id {
		return false
	}
	if fp == nil {
		return true
	}
	return hmac.Equal(fp, key.Fingerprint[:])
}

// KeysById returns the set of keys that have the given key id.
// fp can be optionally supplied, which is the full key fingerprint.
// If it's provided, then it must match. This comes up in the case
// of GPG subpacket 33.
func (el EntityList) KeysById(id uint64, fp []byte) (keys []Key) {
	for _, e := range el {
		if keyMatchesIdAndFingerprint(e.PrimaryKey, id, fp) {
			var selfSig *packet.Signature
			for _, ident := range e.Identities {
				if selfSig == nil {
					selfSig = ident.SelfSignature
				} else if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
					selfSig = ident.SelfSignature
					break
				}
			}

			var keyFlags packet.KeyFlagBits
			for _, ident := range e.Identities {
				keyFlags.Merge(ident.SelfSignature.GetKeyFlags())
			}

			keys = append(keys, Key{e, e.PrimaryKey, e.PrivateKey, selfSig, keyFlags})
		}

		for _, subKey := range e.Subkeys {
			if keyMatchesIdAndFingerprint(subKey.PublicKey, id, fp) {

				// If there's both a a revocation and a sig, then take the
				// revocation. Otherwise, we can proceed with the sig.
				sig := subKey.Revocation
				if sig == nil {
					sig = subKey.Sig
				}

				keys = append(keys, Key{e, subKey.PublicKey, subKey.PrivateKey, sig, sig.GetKeyFlags()})
			}
		}
	}
	return
}

// KeysByIdAndUsage returns the set of keys with the given id that also meet
// the key usage given by requiredUsage.  The requiredUsage is expressed as
// the bitwise-OR of packet.KeyFlag* values.
// fp can be optionally supplied, which is the full key fingerprint.
// If it's provided, then it must match. This comes up in the case
// of GPG subpacket 33.
func (el EntityList) KeysByIdUsage(id uint64, fp []byte, requiredUsage byte) (keys []Key) {
	for _, key := range el.KeysById(id, fp) {
		if len(key.Entity.Revocations) > 0 {
			continue
		}

		if key.SelfSignature.RevocationReason != nil {
			continue
		}

		if requiredUsage != 0 {
			var usage byte

			switch {
			case key.KeyFlags.Valid:
				usage = key.KeyFlags.BitField

			case key.PublicKey.PubKeyAlgo == packet.PubKeyAlgoElGamal:
				// We also need to handle the case where, although the sig's
				// flags aren't valid, the key can is implicitly usable for
				// encryption by virtue of being ElGamal. See also the comment
				// in encryptionKey() above.
				usage |= packet.KeyFlagEncryptCommunications
				usage |= packet.KeyFlagEncryptStorage

			case key.PublicKey.PubKeyAlgo == packet.PubKeyAlgoDSA ||
				key.PublicKey.PubKeyAlgo == packet.PubKeyAlgoECDSA ||
				key.PublicKey.PubKeyAlgo == packet.PubKeyAlgoEdDSA:
				usage |= packet.KeyFlagSign

			// For a primary RSA key without any key flags, be as permissiable
			// as possible.
			case key.PublicKey.PubKeyAlgo == packet.PubKeyAlgoRSA &&
				keyMatchesIdAndFingerprint(key.Entity.PrimaryKey, id, fp):
				usage = (packet.KeyFlagCertify | packet.KeyFlagSign |
					packet.KeyFlagEncryptCommunications | packet.KeyFlagEncryptStorage)
			}

			if usage&requiredUsage != requiredUsage {
				continue
			}
		}

		keys = append(keys, key)
	}
	return
}

// DecryptionKeys returns all private keys that are valid for decryption.
func (el EntityList) DecryptionKeys() (keys []Key) {
	for _, e := range el {
		for _, subKey := range e.Subkeys {
			if subKey.PrivateKey != nil && subKey.PrivateKey.PrivateKey != nil && (!subKey.Sig.FlagsValid || subKey.Sig.FlagEncryptStorage || subKey.Sig.FlagEncryptCommunications) {
				keys = append(keys, Key{e, subKey.PublicKey, subKey.PrivateKey, subKey.Sig, subKey.Sig.GetKeyFlags()})
			}
		}
	}
	return
}

// ReadArmoredKeyRing reads one or more public/private keys from an armor keyring file.
func ReadArmoredKeyRing(r io.Reader) (EntityList, error) {
	block, err := armor.Decode(r)
	if err == io.EOF {
		return nil, errors.InvalidArgumentError("no armored data found")
	}
	if err != nil {
		return nil, err
	}
	if block.Type != PublicKeyType && block.Type != PrivateKeyType {
		return nil, errors.InvalidArgumentError("expected public or private key block, got: " + block.Type)
	}

	return ReadKeyRing(block.Body)
}

// ReadKeyRing reads one or more public/private keys. Unsupported keys are
// ignored as long as at least a single valid key is found.
func ReadKeyRing(r io.Reader) (el EntityList, err error) {
	packets := packet.NewReader(r)
	var lastUnsupportedError error

	for {
		var e *Entity
		e, err = ReadEntity(packets)
		if err != nil {
			// TODO: warn about skipped unsupported/unreadable keys
			if _, ok := err.(errors.UnsupportedError); ok {
				lastUnsupportedError = err
				err = readToNextPublicKey(packets)
			} else if _, ok := err.(errors.StructuralError); ok {
				// Skip unreadable, badly-formatted keys
				lastUnsupportedError = err
				err = readToNextPublicKey(packets)
			}
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				el = nil
				break
			}
		} else {
			el = append(el, e)
		}
	}

	if len(el) == 0 && err == nil {
		err = lastUnsupportedError
	}
	return
}

// readToNextPublicKey reads packets until the start of the entity and leaves
// the first packet of the new entity in the Reader.
func readToNextPublicKey(packets *packet.Reader) (err error) {
	var p packet.Packet
	for {
		p, err = packets.Next()
		if err == io.EOF {
			return
		} else if err != nil {
			if _, ok := err.(errors.UnsupportedError); ok {
				err = nil
				continue
			}
			return
		}

		if pk, ok := p.(*packet.PublicKey); ok && !pk.IsSubkey {
			packets.Unread(p)
			return
		}
	}

	panic("unreachable")
}

// ReadEntity reads an entity (public key, identities, subkeys etc) from the
// given Reader.
func ReadEntity(packets *packet.Reader) (*Entity, error) {
	e := new(Entity)
	e.Identities = make(map[string]*Identity)

	p, err := packets.Next()
	if err != nil {
		return nil, err
	}

	var ok bool
	if e.PrimaryKey, ok = p.(*packet.PublicKey); !ok {
		if e.PrivateKey, ok = p.(*packet.PrivateKey); !ok {
			packets.Unread(p)
			return nil, errors.StructuralError("first packet was not a public/private key")
		} else {
			e.PrimaryKey = &e.PrivateKey.PublicKey
		}
	}

	if !e.PrimaryKey.PubKeyAlgo.CanSign() {
		return nil, errors.StructuralError("primary key cannot be used for signatures")
	}

	var current *Identity
	var revocations []*packet.Signature

	designatedRevokers := make(map[uint64]bool)
EachPacket:
	for {
		p, err := packets.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch pkt := p.(type) {
		case *packet.UserId:

			// Make a new Identity object, that we might wind up throwing away.
			// We'll only add it if we get a valid self-signature over this
			// userID.
			current = new(Identity)
			current.Name = pkt.Id
			current.UserId = pkt
		case *packet.Signature:
			if pkt.SigType == packet.SigTypeKeyRevocation {
				// These revocations won't revoke UIDs (see
				// SigTypeIdentityRevocation). Handle these first,
				// because key might have revocation coming from
				// another key (designated revoke).
				revocations = append(revocations, pkt)
				continue
			}

			// These are signatures by other people on this key. Let's just ignore them
			// from the beginning, since they shouldn't affect our key decoding one way
			// or the other.
			if pkt.IssuerKeyId != nil && *pkt.IssuerKeyId != e.PrimaryKey.KeyId {
				continue
			}

			// If this is a signature made by the keyholder, and the signature has stubbed out
			// critical packets, then *now* we need to bail out.
			if e := pkt.StubbedOutCriticalError; e != nil {
				return nil, e
			}

			// Next handle the case of a self-signature. According to RFC8440,
			// Section 5.2.3.3, if there are several self-signatures,
			// we should take the newer one.  If they were both created
			// at the same time, but one of them has keyflags specified and the
			// other doesn't, keep the one with the keyflags. We have actually
			// seen this in the wild (see the 'Yield' test in read_test.go).
			// If there is a tie, and both have the same value for FlagsValid,
			// then "last writer wins."
			//
			// HOWEVER! We have seen yet more keys in the wild (see the 'Spiros'
			// test in read_test.go), in which the later self-signature is a bunch
			// of junk, and doesn't even specify key flags. Does it really make
			// sense to overwrite reasonable key flags with the empty set? I'm not
			// sure what that would be trying to achieve, and plus GPG seems to be
			// ok with this situation, and ignores the later (empty) keyflag set.
			// So further tighten our overwrite rules, and only allow the later
			// signature to overwrite the earlier signature if so doing won't
			// trash the key flags.
			if current != nil &&
				(current.SelfSignature == nil ||
					(!pkt.CreationTime.Before(current.SelfSignature.CreationTime) &&
						(pkt.FlagsValid || !current.SelfSignature.FlagsValid))) &&
				(pkt.SigType == packet.SigTypePositiveCert || pkt.SigType == packet.SigTypeGenericCert) &&
				pkt.IssuerKeyId != nil &&
				*pkt.IssuerKeyId == e.PrimaryKey.KeyId {

				if err = e.PrimaryKey.VerifyUserIdSignature(current.Name, e.PrimaryKey, pkt); err == nil {

					current.SelfSignature = pkt

					// NOTE(maxtaco) 2016.01.11
					// Only register an identity once we've gotten a valid self-signature.
					// It's possible therefore for us to throw away `current` in the case
					// no valid self-signatures were found. That's OK as long as there are
					// other identities that make sense.
					//
					// NOTE! We might later see a revocation for this very same UID, and it
					// won't be undone. We've preserved this feature from the original
					// Google OpenPGP we forked from.
					e.Identities[current.Name] = current
				} else {
					// We really should warn that there was a failure here. Not raise an error
					// since this really shouldn't be a fail-stop error.
				}
			} else if current != nil && pkt.SigType == packet.SigTypeIdentityRevocation {
				if err = e.PrimaryKey.VerifyUserIdSignature(current.Name, e.PrimaryKey, pkt); err == nil {
					// Note: we are not removing the identity from
					// e.Identities. Caller can always filter by Revocation
					// field to ignore revoked identities.
					current.Revocation = pkt
				}
			} else if pkt.SigType == packet.SigTypeDirectSignature {
				if err = e.PrimaryKey.VerifyRevocationSignature(e.PrimaryKey, pkt); err == nil {
					if desig := pkt.DesignatedRevoker; desig != nil {
						// If it's a designated revoker signature, take last 8 octects
						// of fingerprint as Key ID and save it to designatedRevokers
						// map. We consult this map later to see if a foreign
						// revocation should be added to UnverifiedRevocations.
						keyID := binary.BigEndian.Uint64(desig.Fingerprint[len(desig.Fingerprint)-8:])
						designatedRevokers[keyID] = true
					}
				}
			} else if current == nil {
				// NOTE(maxtaco)
				//
				// See https://github.com/keybase/client/issues/2666
				//
				// There might have been a user attribute picture before this signature,
				// in which case this is still a valid PGP key. In the future we might
				// not ignore user attributes (like picture). But either way, it doesn't
				// make sense to bail out here. Keep looking for other valid signatures.
				//
				// Used to be:
				//    return nil, errors.StructuralError("signature packet found before user id packet")
			} else {
				current.Signatures = append(current.Signatures, pkt)
			}
		case *packet.PrivateKey:
			if pkt.IsSubkey == false {
				packets.Unread(p)
				break EachPacket
			}
			err = addSubkey(e, packets, &pkt.PublicKey, pkt)
			if err != nil {
				return nil, err
			}
		case *packet.PublicKey:
			if pkt.IsSubkey == false {
				packets.Unread(p)
				break EachPacket
			}
			err = addSubkey(e, packets, pkt, nil)
			if err != nil {
				return nil, err
			}
		default:
			// we ignore unknown packets
		}
	}

	if len(e.Identities) == 0 {
		return nil, errors.StructuralError("entity without any identities")
	}

	for _, revocation := range revocations {
		if revocation.IssuerKeyId == nil || *revocation.IssuerKeyId == e.PrimaryKey.KeyId {
			// Key revokes itself, something that we can verify.
			err = e.PrimaryKey.VerifyRevocationSignature(e.PrimaryKey, revocation)
			if err == nil {
				e.Revocations = append(e.Revocations, revocation)
			} else {
				return nil, errors.StructuralError("revocation signature signed by alternate key")
			}
		} else if revocation.IssuerKeyId != nil {
			if _, ok := designatedRevokers[*revocation.IssuerKeyId]; ok {
				// Revocation is done by certified designated revoker,
				// but we can't verify the revocation.
				e.UnverifiedRevocations = append(e.UnverifiedRevocations, revocation)
			}
		}
	}

	return e, nil
}

func addSubkey(e *Entity, packets *packet.Reader, pub *packet.PublicKey, priv *packet.PrivateKey) error {
	var subKey Subkey
	subKey.PublicKey = pub
	subKey.PrivateKey = priv
	var lastErr error
	for {
		p, err := packets.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.StructuralError("subkey signature invalid: " + err.Error())
		}
		sig, ok := p.(*packet.Signature)
		if !ok {
			// Hit a non-signature packet, so assume we're up to the next key
			packets.Unread(p)
			break
		}
		if st := sig.SigType; st != packet.SigTypeSubkeyBinding && st != packet.SigTypeSubkeyRevocation {

			// Note(maxtaco):
			// We used to error out here, but instead, let's fast-forward past
			// packets that are in the wrong place (like misplaced 0x13 signatures)
			// until we get to one that works.  For a test case,
			// see TestWithBadSubkeySignaturePackets.

			continue
		}
		err = e.PrimaryKey.VerifyKeySignature(subKey.PublicKey, sig)
		if err != nil {
			// Non valid signature, so again, no need to abandon all hope, just continue;
			// make a note of the error we hit.
			lastErr = errors.StructuralError("subkey signature invalid: " + err.Error())
			continue
		}
		switch sig.SigType {
		case packet.SigTypeSubkeyBinding:
			// Does the "new" sig set expiration to later date than
			// "previous" sig?
			if subKey.Sig == nil || subKey.Sig.ExpiresBeforeOther(sig) {
				subKey.Sig = sig
			}
		case packet.SigTypeSubkeyRevocation:
			// First writer wins
			if subKey.Revocation == nil {
				subKey.Revocation = sig
			}
		}
	}

	if subKey.Sig != nil {
		if err := subKey.PublicKey.ErrorIfDeprecated(); err != nil {
			// Key passed signature check but is deprecated.
			subKey.Sig = nil
			lastErr = err
		}
	}

	if subKey.Sig != nil {
		e.Subkeys = append(e.Subkeys, subKey)
	} else {
		if lastErr == nil {
			lastErr = errors.StructuralError("Subkey wasn't signed; expected a 'binding' signature")
		}
		e.BadSubkeys = append(e.BadSubkeys, BadSubkey{Subkey: subKey, Err: lastErr})
	}
	return nil
}

const defaultRSAKeyBits = 2048

// NewEntity returns an Entity that contains a fresh RSA/RSA keypair with a
// single identity composed of the given full name, comment and email, any of
// which may be empty but must not contain any of "()<>\x00".
// If config is nil, sensible defaults will be used.
func NewEntity(name, comment, email string, config *packet.Config) (*Entity, error) {
	currentTime := config.Now()

	bits := defaultRSAKeyBits
	if config != nil && config.RSABits != 0 {
		bits = config.RSABits
	}

	uid := packet.NewUserId(name, comment, email)
	if uid == nil {
		return nil, errors.InvalidArgumentError("user id field contained invalid characters")
	}
	signingPriv, err := rsa.GenerateKey(config.Random(), bits)
	if err != nil {
		return nil, err
	}
	encryptingPriv, err := rsa.GenerateKey(config.Random(), bits)
	if err != nil {
		return nil, err
	}

	e := &Entity{
		PrimaryKey: packet.NewRSAPublicKey(currentTime, &signingPriv.PublicKey),
		PrivateKey: packet.NewRSAPrivateKey(currentTime, signingPriv),
		Identities: make(map[string]*Identity),
	}
	isPrimaryId := true
	e.Identities[uid.Id] = &Identity{
		Name:   uid.Id,
		UserId: uid,
		SelfSignature: &packet.Signature{
			CreationTime: currentTime,
			SigType:      packet.SigTypePositiveCert,
			PubKeyAlgo:   packet.PubKeyAlgoRSA,
			Hash:         config.Hash(),
			IsPrimaryId:  &isPrimaryId,
			FlagsValid:   true,
			FlagSign:     true,
			FlagCertify:  true,
			IssuerKeyId:  &e.PrimaryKey.KeyId,
		},
	}

	// If the user passes in a DefaultHash via packet.Config, set the
	// PreferredHash for the SelfSignature.
	if config != nil && config.DefaultHash != 0 {
		e.Identities[uid.Id].SelfSignature.PreferredHash = []uint8{hashToHashId(config.DefaultHash)}
	}

	// Likewise for DefaultCipher.
	if config != nil && config.DefaultCipher != 0 {
		e.Identities[uid.Id].SelfSignature.PreferredSymmetric = []uint8{uint8(config.DefaultCipher)}
	}

	e.Subkeys = make([]Subkey, 1)
	e.Subkeys[0] = Subkey{
		PublicKey:  packet.NewRSAPublicKey(currentTime, &encryptingPriv.PublicKey),
		PrivateKey: packet.NewRSAPrivateKey(currentTime, encryptingPriv),
		Sig: &packet.Signature{
			CreationTime:              currentTime,
			SigType:                   packet.SigTypeSubkeyBinding,
			PubKeyAlgo:                packet.PubKeyAlgoRSA,
			Hash:                      config.Hash(),
			FlagsValid:                true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &e.PrimaryKey.KeyId,
		},
	}
	e.Subkeys[0].PublicKey.IsSubkey = true
	e.Subkeys[0].PrivateKey.IsSubkey = true

	return e, nil
}

// SerializePrivate serializes an Entity, including private key material, to
// the given Writer. For now, it must only be used on an Entity returned from
// NewEntity.
// If config is nil, sensible defaults will be used.
func (e *Entity) SerializePrivate(w io.Writer, config *packet.Config) (err error) {
	err = e.PrivateKey.Serialize(w)
	if err != nil {
		return
	}
	for _, ident := range e.Identities {
		err = ident.UserId.Serialize(w)
		if err != nil {
			return
		}
		if e.PrivateKey.PrivateKey != nil {
			err = ident.SelfSignature.SignUserId(ident.UserId.Id, e.PrimaryKey, e.PrivateKey, config)
			if err != nil {
				return
			}
		}
		err = ident.SelfSignature.Serialize(w)
		if err != nil {
			return
		}
	}
	for _, subkey := range e.Subkeys {
		err = subkey.PrivateKey.Serialize(w)
		if err != nil {
			return
		}
		if e.PrivateKey.PrivateKey != nil && !config.ReuseSignatures() {
			// If not reusing existing signatures, sign subkey using private key
			// (subkey binding), but also sign primary key using subkey (primary
			// key binding) if subkey is used for signing.
			if subkey.Sig.FlagSign {
				err = subkey.Sig.CrossSignKey(e.PrimaryKey, subkey.PrivateKey, config)
				if err != nil {
					return err
				}
			}
			err = subkey.Sig.SignKey(subkey.PublicKey, e.PrivateKey, config)
			if err != nil {
				return
			}
		}

		if subkey.Revocation != nil {
			err = subkey.Revocation.Serialize(w)
			if err != nil {
				return
			}
		}

		err = subkey.Sig.Serialize(w)
		if err != nil {
			return
		}
	}
	return nil
}

// Serialize writes the public part of the given Entity to w. (No private
// key material will be output).
func (e *Entity) Serialize(w io.Writer) error {
	err := e.PrimaryKey.Serialize(w)
	if err != nil {
		return err
	}
	for _, ident := range e.Identities {
		err = ident.UserId.Serialize(w)
		if err != nil {
			return err
		}
		err = ident.SelfSignature.Serialize(w)
		if err != nil {
			return err
		}
		for _, sig := range ident.Signatures {
			err = sig.Serialize(w)
			if err != nil {
				return err
			}
		}
	}
	for _, subkey := range e.Subkeys {
		err = subkey.PublicKey.Serialize(w)
		if err != nil {
			return err
		}

		if subkey.Revocation != nil {
			err = subkey.Revocation.Serialize(w)
			if err != nil {
				return err
			}
		}
		err = subkey.Sig.Serialize(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// SignIdentity adds a signature to e, from signer, attesting that identity is
// associated with e. The provided identity must already be an element of
// e.Identities and the private key of signer must have been decrypted if
// necessary.
// If config is nil, sensible defaults will be used.
func (e *Entity) SignIdentity(identity string, signer *Entity, config *packet.Config) error {
	if signer.PrivateKey == nil {
		return errors.InvalidArgumentError("signing Entity must have a private key")
	}
	if signer.PrivateKey.Encrypted {
		return errors.InvalidArgumentError("signing Entity's private key must be decrypted")
	}
	ident, ok := e.Identities[identity]
	if !ok {
		return errors.InvalidArgumentError("given identity string not found in Entity")
	}

	sig := &packet.Signature{
		SigType:      packet.SigTypeGenericCert,
		PubKeyAlgo:   signer.PrivateKey.PubKeyAlgo,
		Hash:         config.Hash(),
		CreationTime: config.Now(),
		IssuerKeyId:  &signer.PrivateKey.KeyId,
	}
	if err := sig.SignUserId(identity, e.PrimaryKey, signer.PrivateKey, config); err != nil {
		return err
	}
	ident.Signatures = append(ident.Signatures, sig)
	return nil
}

// CopySubkeyRevocations copies subkey revocations from the src Entity over
// to the receiver entity. We need this because `gpg --export-secret-key` does
// not appear to output subkey revocations.  In this case we need to manually
// merge with the output of `gpg --export`.
func (e *Entity) CopySubkeyRevocations(src *Entity) {
	m := make(map[[20]byte]*packet.Signature)
	for _, subkey := range src.Subkeys {
		if subkey.Revocation != nil {
			m[subkey.PublicKey.Fingerprint] = subkey.Revocation
		}
	}
	for i, subkey := range e.Subkeys {
		if r := m[subkey.PublicKey.Fingerprint]; r != nil {
			e.Subkeys[i].Revocation = r
		}
	}
}

// CheckDesignatedRevokers will try to confirm any of designated
// revocation of entity. For this function to work, revocation
// issuer's key should be found in keyring. First successfully
// verified designated revocation is returned along with the key that
// verified it.
func FindVerifiedDesignatedRevoke(keyring KeyRing, entity *Entity) (*packet.Signature, *Key) {
	for _, sig := range entity.UnverifiedRevocations {
		if sig.IssuerKeyId == nil {
			continue
		}

		issuerKeyId := *sig.IssuerKeyId
		issuerFingerprint := sig.IssuerFingerprint
		keys := keyring.KeysByIdUsage(issuerKeyId, issuerFingerprint, packet.KeyFlagSign)
		if len(keys) == 0 {
			continue
		}
		for _, key := range keys {
			err := key.PublicKey.VerifyRevocationSignature(entity.PrimaryKey, sig)
			if err == nil {
				return sig, &key
			}
		}
	}

	return nil, nil
}
