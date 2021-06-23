package lz4block

import (
	"encoding/binary"
)

func decodeBlockGo(dst, src, dict []byte) (ret int) {
	// Restrict capacities so we don't read or write out of bounds.
	dst = dst[:len(dst):len(dst)]
	src = src[:len(src):len(src)]
	dictLen := uint(len(dict))

	const hasError = -2
	defer func() {
		if recover() != nil {
			ret = hasError
		}
	}()

	var si, di uint
	for {
		// Literals and match lengths (token).
		b := uint(src[si])
		si++

		// Literals.
		if lLen := b >> 4; lLen > 0 {
			switch {
			case lLen < 0xF && si+16 < uint(len(src)):
				// Shortcut 1
				// if we have enough room in src and dst, and the literals length
				// is small enough (0..14) then copy all 16 bytes, even if not all
				// are part of the literals.
				copy(dst[di:], src[si:si+16])
				si += lLen
				di += lLen
				if mLen := b & 0xF; mLen < 0xF {
					// Shortcut 2
					// if the match length (4..18) fits within the literals, then copy
					// all 18 bytes, even if not all are part of the literals.
					mLen += 4
					if offset := u16(src[si:]); mLen <= offset && offset < di {
						i := di - offset
						end := i + 18
						if end > uint(len(dst)) {
							// The remaining buffer may not hold 18 bytes.
							// See https://github.com/pierrec/lz4/issues/51.
							end = uint(len(dst))
						}
						copy(dst[di:], dst[i:end])
						si += 2
						di += mLen
						continue
					}
				}
			case lLen == 0xF:
				for src[si] == 0xFF {
					lLen += 0xFF
					si++
				}
				lLen += uint(src[si])
				si++
				fallthrough
			default:
				copy(dst[di:di+lLen], src[si:si+lLen])
				si += lLen
				di += lLen
			}
		}
		if si == uint(len(src)) {
			return int(di)
		} else if si > uint(len(src)) {
			return hasError
		}

		offset := u16(src[si:])
		if offset == 0 {
			return hasError
		}
		si += 2

		// Match.
		mLen := b & 0xF
		if mLen == 0xF {
			for src[si] == 0xFF {
				mLen += 0xFF
				si++
			}
			mLen += uint(src[si])
			si++
		}
		mLen += minMatch

		// Copy the match.
		if di < offset {
			// The match is beyond our block, meaning in the dictionary
			if offset-di > mLen {
				// The match is entirely contained in the dictionary. Just copy!
				copy(dst[di:di+mLen], dict[dictLen+di-offset:dictLen+di-offset+mLen])
				di = di + mLen
			} else {
				// The match stretches over the dictionary and our block
				copySize := offset - di
				restSize := mLen - copySize

				copy(dst[di:di+copySize], dict[dictLen-copySize:])
				di = di + copySize

				if di < restSize {
					// Overlap - we want to copy more than what we have available,
					// so copy byte per byte.
					copyFrom := 0
					endOfMatch := di + restSize
					for di < endOfMatch {
						dst[di] = dst[copyFrom]
						di = di + 1
						copyFrom = copyFrom + 1
					}
				} else {
					copy(dst[di:di+restSize], dst[0:restSize])
					di = di + restSize
				}
			}
			continue
		}

		expanded := dst[di-offset:]
		if mLen > offset {
			// Efficiently copy the match dst[di-offset:di] into the dst slice.
			bytesToCopy := offset * (mLen / offset)
			for n := offset; n <= bytesToCopy+offset; n *= 2 {
				copy(expanded[n:], expanded[:n])
			}
			di += bytesToCopy
			mLen -= bytesToCopy
		}
		di += uint(copy(dst[di:di+mLen], expanded[:mLen]))
	}
}

func u16(p []byte) uint { return uint(binary.LittleEndian.Uint16(p)) }
