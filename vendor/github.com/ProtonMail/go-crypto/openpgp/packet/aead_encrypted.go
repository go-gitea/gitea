// Copyright (C) 2019 ProtonTech AG

package packet

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
)

// AEADEncrypted represents an AEAD Encrypted Packet (tag 20, RFC4880bis-5.16).
type AEADEncrypted struct {
	cipher        CipherFunction
	mode          AEADMode
	chunkSizeByte byte
	Contents      io.Reader // Encrypted chunks and tags
	initialNonce  []byte    // Referred to as IV in RFC4880-bis
}

// Only currently defined version
const aeadEncryptedVersion = 1

// An AEAD opener/sealer, its configuration, and data for en/decryption.
type aeadCrypter struct {
	aead           cipher.AEAD
	chunkSize      int
	initialNonce   []byte
	associatedData []byte       // Chunk-independent associated data
	chunkIndex     []byte       // Chunk counter
	bytesProcessed int          // Amount of plaintext bytes encrypted/decrypted
	buffer         bytes.Buffer // Buffered bytes accross chunks
}

// aeadEncrypter encrypts and writes bytes. It encrypts when necessary according
// to the AEAD block size, and buffers the extra encrypted bytes for next write.
type aeadEncrypter struct {
	aeadCrypter                // Embedded plaintext sealer
	writer      io.WriteCloser // 'writer' is a partialLengthWriter
}

// aeadDecrypter reads and decrypts bytes. It buffers extra decrypted bytes when
// necessary, similar to aeadEncrypter.
type aeadDecrypter struct {
	aeadCrypter           // Embedded ciphertext opener
	reader      io.Reader // 'reader' is a partialLengthReader
	peekedBytes []byte    // Used to detect last chunk
	eof         bool
}

func (ae *AEADEncrypted) parse(buf io.Reader) error {
	headerData := make([]byte, 4)
	if n, err := io.ReadFull(buf, headerData); n < 4 {
		return errors.AEADError("could not read aead header:" + err.Error())
	}
	// Read initial nonce
	mode := AEADMode(headerData[2])
	nonceLen := mode.NonceLength()
	if nonceLen == 0 {
		return errors.AEADError("unknown mode")
	}
	initialNonce := make([]byte, nonceLen)
	if n, err := io.ReadFull(buf, initialNonce); n < nonceLen {
		return errors.AEADError("could not read aead nonce:" + err.Error())
	}
	ae.Contents = buf
	ae.initialNonce = initialNonce
	c := headerData[1]
	if _, ok := algorithm.CipherById[c]; !ok {
		return errors.UnsupportedError("unknown cipher: " + string(c))
	}
	ae.cipher = CipherFunction(c)
	ae.mode = mode
	ae.chunkSizeByte = byte(headerData[3])
	return nil
}

// Decrypt returns a io.ReadCloser from which decrypted bytes can be read, or
// an error.
func (ae *AEADEncrypted) Decrypt(ciph CipherFunction, key []byte) (io.ReadCloser, error) {
	return ae.decrypt(key)
}

// decrypt prepares an aeadCrypter and returns a ReadCloser from which
// decrypted bytes can be read (see aeadDecrypter.Read()).
func (ae *AEADEncrypted) decrypt(key []byte) (io.ReadCloser, error) {
	blockCipher := ae.cipher.new(key)
	aead := ae.mode.new(blockCipher)
	// Carry the first tagLen bytes
	tagLen := ae.mode.TagLength()
	peekedBytes := make([]byte, tagLen)
	n, err := io.ReadFull(ae.Contents, peekedBytes)
	if n < tagLen || (err != nil && err != io.EOF) {
		return nil, errors.AEADError("Not enough data to decrypt:" + err.Error())
	}
	chunkSize := decodeAEADChunkSize(ae.chunkSizeByte)
	return &aeadDecrypter{
		aeadCrypter: aeadCrypter{
			aead:           aead,
			chunkSize:      chunkSize,
			initialNonce:   ae.initialNonce,
			associatedData: ae.associatedData(),
			chunkIndex:     make([]byte, 8),
		},
		reader:      ae.Contents,
		peekedBytes: peekedBytes}, nil
}

// Read decrypts bytes and reads them into dst. It decrypts when necessary and
// buffers extra decrypted bytes. It returns the number of bytes copied into dst
// and an error.
func (ar *aeadDecrypter) Read(dst []byte) (n int, err error) {
	// Return buffered plaintext bytes from previous calls
	if ar.buffer.Len() > 0 {
		return ar.buffer.Read(dst)
	}

	// Return EOF if we've previously validated the final tag
	if ar.eof {
		return 0, io.EOF
	}

	// Read a chunk
	tagLen := ar.aead.Overhead()
	cipherChunkBuf := new(bytes.Buffer)
	_, errRead := io.CopyN(cipherChunkBuf, ar.reader, int64(ar.chunkSize + tagLen))
	cipherChunk := cipherChunkBuf.Bytes()
	if errRead != nil && errRead != io.EOF {
		return 0, errRead
	}
	decrypted, errChunk := ar.openChunk(cipherChunk)
	if errChunk != nil {
		return 0, errChunk
	}

	// Return decrypted bytes, buffering if necessary
	if len(dst) < len(decrypted) {
		n = copy(dst, decrypted[:len(dst)])
		ar.buffer.Write(decrypted[len(dst):])
	} else {
		n = copy(dst, decrypted)
	}

	// Check final authentication tag
	if errRead == io.EOF {
		errChunk := ar.validateFinalTag(ar.peekedBytes)
		if errChunk != nil {
			return n, errChunk
		}
		ar.eof = true // Mark EOF for when we've returned all buffered data
	}
	return
}

// Close is noOp. The final authentication tag of the stream was already
// checked in the last Read call. In the future, this function could be used to
// wipe the reader and peeked, decrypted bytes, if necessary.
func (ar *aeadDecrypter) Close() (err error) {
	return nil
}

// SerializeAEADEncrypted initializes the aeadCrypter and returns a writer.
// This writer encrypts and writes bytes (see aeadEncrypter.Write()).
func SerializeAEADEncrypted(w io.Writer, key []byte, cipher CipherFunction, mode AEADMode, config *Config) (io.WriteCloser, error) {
	writeCloser := noOpCloser{w}
	writer, err := serializeStreamHeader(writeCloser, packetTypeAEADEncrypted)
	if err != nil {
		return nil, err
	}

	// Data for en/decryption: tag, version, cipher, aead mode, chunk size
	aeadConf := config.AEAD()
	prefix := []byte{
		0xD4,
		aeadEncryptedVersion,
		byte(config.Cipher()),
		byte(aeadConf.Mode()),
		aeadConf.ChunkSizeByte(),
	}
	n, err := writer.Write(prefix[1:])
	if err != nil || n < 4 {
		return nil, errors.AEADError("could not write AEAD headers")
	}
	// Sample nonce
	nonceLen := aeadConf.Mode().NonceLength()
	nonce := make([]byte, nonceLen)
	n, err = rand.Read(nonce)
	if err != nil {
		panic("Could not sample random nonce")
	}
	_, err = writer.Write(nonce)
	if err != nil {
		return nil, err
	}
	blockCipher := CipherFunction(config.Cipher()).new(key)
	alg := AEADMode(aeadConf.Mode()).new(blockCipher)

	chunkSize := decodeAEADChunkSize(aeadConf.ChunkSizeByte())
	return &aeadEncrypter{
		aeadCrypter: aeadCrypter{
			aead:           alg,
			chunkSize:      chunkSize,
			associatedData: prefix,
			chunkIndex:     make([]byte, 8),
			initialNonce:   nonce,
		},
		writer: writer}, nil
}

// Write encrypts and writes bytes. It encrypts when necessary and buffers extra
// plaintext bytes for next call. When the stream is finished, Close() MUST be
// called to append the final tag.
func (aw *aeadEncrypter) Write(plaintextBytes []byte) (n int, err error) {
	// Append plaintextBytes to existing buffered bytes
	n, err = aw.buffer.Write(plaintextBytes)
	if err != nil {
		return n, err
	}
	// Encrypt and write chunks
	for aw.buffer.Len() >= aw.chunkSize {
		plainChunk := aw.buffer.Next(aw.chunkSize)
		encryptedChunk, err := aw.sealChunk(plainChunk)
		if err != nil {
			return n, err
		}
		_, err = aw.writer.Write(encryptedChunk)
		if err != nil {
			return n, err
		}
	}
	return
}

// Close encrypts and writes the remaining buffered plaintext if any, appends
// the final authentication tag, and closes the embedded writer. This function
// MUST be called at the end of a stream.
func (aw *aeadEncrypter) Close() (err error) {
	// Encrypt and write a chunk if there's buffered data left, or if we haven't
	// written any chunks yet.
	if aw.buffer.Len() > 0 || aw.bytesProcessed == 0 {
		plainChunk := aw.buffer.Bytes()
		lastEncryptedChunk, err := aw.sealChunk(plainChunk)
		if err != nil {
			return err
		}
		_, err = aw.writer.Write(lastEncryptedChunk)
		if err != nil {
			return err
		}
	}
	// Compute final tag (associated data: packet tag, version, cipher, aead,
	// chunk size, index, total number of encrypted octets).
	adata := append(aw.associatedData[:], aw.chunkIndex[:]...)
	adata = append(adata, make([]byte, 8)...)
	binary.BigEndian.PutUint64(adata[13:], uint64(aw.bytesProcessed))
	nonce := aw.computeNextNonce()
	finalTag := aw.aead.Seal(nil, nonce, nil, adata)
	_, err = aw.writer.Write(finalTag)
	if err != nil {
		return err
	}
	return aw.writer.Close()
}

// sealChunk Encrypts and authenticates the given chunk.
func (aw *aeadEncrypter) sealChunk(data []byte) ([]byte, error) {
	if len(data) > aw.chunkSize {
		return nil, errors.AEADError("chunk exceeds maximum length")
	}
	if aw.associatedData == nil {
		return nil, errors.AEADError("can't seal without headers")
	}
	adata := append(aw.associatedData, aw.chunkIndex...)
	nonce := aw.computeNextNonce()
	encrypted := aw.aead.Seal(nil, nonce, data, adata)
	aw.bytesProcessed += len(data)
	if err := aw.aeadCrypter.incrementIndex(); err != nil {
		return nil, err
	}
	return encrypted, nil
}

// openChunk decrypts and checks integrity of an encrypted chunk, returning
// the underlying plaintext and an error. It access peeked bytes from next
// chunk, to identify the last chunk and decrypt/validate accordingly.
func (ar *aeadDecrypter) openChunk(data []byte) ([]byte, error) {
	tagLen := ar.aead.Overhead()
	// Restore carried bytes from last call
	chunkExtra := append(ar.peekedBytes, data...)
	// 'chunk' contains encrypted bytes, followed by an authentication tag.
	chunk := chunkExtra[:len(chunkExtra)-tagLen]
	ar.peekedBytes = chunkExtra[len(chunkExtra)-tagLen:]
	adata := append(ar.associatedData, ar.chunkIndex...)
	nonce := ar.computeNextNonce()
	plainChunk, err := ar.aead.Open(nil, nonce, chunk, adata)
	if err != nil {
		return nil, err
	}
	ar.bytesProcessed += len(plainChunk)
	if err = ar.aeadCrypter.incrementIndex(); err != nil {
		return nil, err
	}
	return plainChunk, nil
}

// Checks the summary tag. It takes into account the total decrypted bytes into
// the associated data. It returns an error, or nil if the tag is valid.
func (ar *aeadDecrypter) validateFinalTag(tag []byte) error {
	// Associated: tag, version, cipher, aead, chunk size, index, and octets
	amountBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(amountBytes, uint64(ar.bytesProcessed))
	adata := append(ar.associatedData, ar.chunkIndex...)
	adata = append(adata, amountBytes...)
	nonce := ar.computeNextNonce()
	_, err := ar.aead.Open(nil, nonce, tag, adata)
	if err != nil {
		return err
	}
	return nil
}

// Associated data for chunks: tag, version, cipher, mode, chunk size byte
func (ae *AEADEncrypted) associatedData() []byte {
	return []byte{
		0xD4,
		aeadEncryptedVersion,
		byte(ae.cipher),
		byte(ae.mode),
		ae.chunkSizeByte}
}

// computeNonce takes the incremental index and computes an eXclusive OR with
// the least significant 8 bytes of the receivers' initial nonce (see sec.
// 5.16.1 and 5.16.2). It returns the resulting nonce.
func (wo *aeadCrypter) computeNextNonce() (nonce []byte) {
	nonce = make([]byte, len(wo.initialNonce))
	copy(nonce, wo.initialNonce)
	offset := len(wo.initialNonce) - 8
	for i := 0; i < 8; i++ {
		nonce[i+offset] ^= wo.chunkIndex[i]
	}
	return
}

// incrementIndex perfoms an integer increment by 1 of the integer represented by the
// slice, modifying it accordingly.
func (wo *aeadCrypter) incrementIndex() error {
	index := wo.chunkIndex
	if len(index) == 0 {
		return errors.AEADError("Index has length 0")
	}
	for i := len(index) - 1; i >= 0; i-- {
		if index[i] < 255 {
			index[i]++
			return nil
		}
		index[i] = 0
	}
	return errors.AEADError("cannot further increment index")
}
