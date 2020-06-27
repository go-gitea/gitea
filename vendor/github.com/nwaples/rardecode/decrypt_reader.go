package rardecode

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
)

// cipherBlockReader implements Block Mode decryption of an io.Reader object.
type cipherBlockReader struct {
	r      io.Reader
	mode   cipher.BlockMode
	inbuf  []byte // input buffer for partial data block
	outbuf []byte // output buffer used when output slice < block size
	n      int    // bytes read from outbuf
	err    error
}

// read reads and decrypts one or more input blocks into p.
// len(p) must be >= cipher block size.
func (cr *cipherBlockReader) read(p []byte) (n int, err error) {
	bs := cr.mode.BlockSize()
	// round p down to a multiple of the block size
	l := len(p) - len(p)%bs
	p = p[:l]

	l = len(cr.inbuf)
	if l > 0 {
		// copy any buffered input into p
		copy(p, cr.inbuf)
		cr.inbuf = cr.inbuf[:0]
	}
	// read data for at least one block
	n, err = io.ReadAtLeast(cr.r, p[l:], bs-l)
	n += l
	p = p[:n]

	l = n % bs
	// check if p is a multiple of the cipher block size
	if l > 0 {
		n -= l
		// save trailing partial block to process later
		cr.inbuf = append(cr.inbuf, p[n:]...)
		p = p[:n]
	}

	if err != nil {
		if err == io.ErrUnexpectedEOF || err == io.ErrShortBuffer {
			// ignore trailing bytes < block size length
			err = io.EOF
		}
		return 0, err
	}
	cr.mode.CryptBlocks(p, p) // decrypt block(s)
	return n, nil
}

// Read reads and decrypts data into p.
// If the input is not a multiple of the cipher block size,
// the trailing bytes will be ignored.
func (cr *cipherBlockReader) Read(p []byte) (n int, err error) {
	for {
		if cr.n < len(cr.outbuf) {
			// return buffered output
			n = copy(p, cr.outbuf[cr.n:])
			cr.n += n
			return n, nil
		}
		if cr.err != nil {
			err = cr.err
			cr.err = nil
			return 0, err
		}
		if len(p) >= cap(cr.outbuf) {
			break
		}
		// p is not large enough to process a block, use outbuf instead
		n, cr.err = cr.read(cr.outbuf[:cap(cr.outbuf)])
		cr.outbuf = cr.outbuf[:n]
		cr.n = 0
	}
	// read blocks into p
	return cr.read(p)
}

// ReadByte returns the next decrypted byte.
func (cr *cipherBlockReader) ReadByte() (byte, error) {
	for {
		if cr.n < len(cr.outbuf) {
			c := cr.outbuf[cr.n]
			cr.n++
			return c, nil
		}
		if cr.err != nil {
			err := cr.err
			cr.err = nil
			return 0, err
		}
		// refill outbuf
		var n int
		n, cr.err = cr.read(cr.outbuf[:cap(cr.outbuf)])
		cr.outbuf = cr.outbuf[:n]
		cr.n = 0
	}
}

// newCipherBlockReader returns a cipherBlockReader that decrypts the given io.Reader using
// the provided block mode cipher.
func newCipherBlockReader(r io.Reader, mode cipher.BlockMode) *cipherBlockReader {
	cr := &cipherBlockReader{r: r, mode: mode}
	cr.outbuf = make([]byte, 0, mode.BlockSize())
	cr.inbuf = make([]byte, 0, mode.BlockSize())
	return cr
}

// newAesDecryptReader returns a cipherBlockReader that decrypts input from a given io.Reader using AES.
// It will panic if the provided key is invalid.
func newAesDecryptReader(r io.Reader, key, iv []byte) *cipherBlockReader {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	mode := cipher.NewCBCDecrypter(block, iv)

	return newCipherBlockReader(r, mode)
}
