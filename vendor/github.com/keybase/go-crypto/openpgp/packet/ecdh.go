package packet

import (
	"bytes"
	"io"
	"math/big"

	"github.com/keybase/go-crypto/openpgp/ecdh"
	"github.com/keybase/go-crypto/openpgp/errors"
	"github.com/keybase/go-crypto/openpgp/s2k"
)

// ECDHKdfParams generates KDF parameters sequence for given
// PublicKey. See https://tools.ietf.org/html/rfc6637#section-8
func ECDHKdfParams(pub *PublicKey) []byte {
	buf := new(bytes.Buffer)
	oid := pub.ec.oid
	buf.WriteByte(byte(len(oid)))
	buf.Write(oid)
	buf.WriteByte(18) // ECDH TYPE
	pub.ecdh.serialize(buf)
	buf.WriteString("Anonymous Sender    ")
	buf.Write(pub.Fingerprint[:])
	return buf.Bytes()
}

func decryptKeyECDH(priv *PrivateKey, X, Y *big.Int, C []byte) (out []byte, err error) {
	ecdhpriv, ok := priv.PrivateKey.(*ecdh.PrivateKey)
	if !ok {
		return nil, errors.InvalidArgumentError("bad internal ECDH key")
	}

	Sx := ecdhpriv.DecryptShared(X, Y)

	kdfParams := ECDHKdfParams(&priv.PublicKey)
	hash, ok := s2k.HashIdToHash(byte(priv.ecdh.KdfHash))
	if !ok {
		return nil, errors.InvalidArgumentError("invalid hash id in private key")
	}

	key := ecdhpriv.KDF(Sx, kdfParams, hash)
	keySize := CipherFunction(priv.ecdh.KdfAlgo).KeySize()

	decrypted, err := ecdh.AESKeyUnwrap(key[:keySize], C)
	if err != nil {
		return nil, err
	}

	// We have to "read ahead" to discover real length of the
	// encryption key and properly unpad buffer.
	cipherFunc := CipherFunction(decrypted[0])
	// +3 bytes = 1-byte cipher id and checksum 2-byte checksum.
	out = ecdh.UnpadBuffer(decrypted, cipherFunc.KeySize()+3)
	if out == nil {
		return nil, errors.InvalidArgumentError("invalid padding while ECDH")
	}
	return out, nil
}

func serializeEncryptedKeyECDH(w io.Writer, rand io.Reader, header [10]byte, pub *PublicKey, keyBlock []byte) error {
	ecdhpub := pub.PublicKey.(*ecdh.PublicKey)
	kdfParams := ECDHKdfParams(pub)

	hash, ok := s2k.HashIdToHash(byte(pub.ecdh.KdfHash))
	if !ok {
		return errors.InvalidArgumentError("invalid hash id in private key")
	}

	kdfKeySize := CipherFunction(pub.ecdh.KdfAlgo).KeySize()
	Vx, Vy, C, err := ecdhpub.Encrypt(rand, kdfParams, keyBlock, hash, kdfKeySize)
	if err != nil {
		return err
	}

	mpis, mpiBitLen := ecdh.Marshal(ecdhpub.Curve, Vx, Vy)

	packetLen := len(header) /* header length in bytes */
	packetLen += 2 /* mpi length in bits */ + len(mpis)
	packetLen += 1 /* ciphertext size in bytes */ + len(C)

	err = serializeHeader(w, packetTypeEncryptedKey, packetLen)
	if err != nil {
		return err
	}

	_, err = w.Write(header[:])
	if err != nil {
		return err
	}

	_, err = w.Write([]byte{byte(mpiBitLen >> 8), byte(mpiBitLen)})
	if err != nil {
		return err
	}

	_, err = w.Write(mpis[:])
	if err != nil {
		return err
	}

	w.Write([]byte{byte(len(C))})
	w.Write(C[:])
	return nil
}
