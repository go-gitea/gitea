package brotli

/* Copyright 2010 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Write bits into a byte array. */

/* This function writes bits into bytes in increasing addresses, and within
   a byte least-significant-bit first.

   The function can write up to 56 bits in one go with WriteBits
   Example: let's assume that 3 bits (Rs below) have been written already:

   BYTE-0     BYTE+1       BYTE+2

   0000 0RRR    0000 0000    0000 0000

   Now, we could write 5 or less bits in MSB by just sifting by 3
   and OR'ing to BYTE-0.

   For n bits, we take the last 5 bits, OR that with high bits in BYTE-0,
   and locate the rest in BYTE+1, BYTE+2, etc. */
func writeBits(n_bits uint, bits uint64, pos *uint, array []byte) {
	var array_pos []byte = array[*pos>>3:]
	var bits_reserved_in_first_byte uint = (*pos & 7)
	/* implicit & 0xFF is assumed for uint8_t arithmetics */

	var bits_left_to_write uint
	bits <<= bits_reserved_in_first_byte
	array_pos[0] |= byte(bits)
	array_pos = array_pos[1:]
	for bits_left_to_write = n_bits + bits_reserved_in_first_byte; bits_left_to_write >= 9; bits_left_to_write -= 8 {
		bits >>= 8
		array_pos[0] = byte(bits)
		array_pos = array_pos[1:]
	}

	array_pos[0] = 0
	*pos += n_bits
}

func writeSingleBit(bit bool, pos *uint, array []byte) {
	if bit {
		writeBits(1, 1, pos, array)
	} else {
		writeBits(1, 0, pos, array)
	}
}

func writeBitsPrepareStorage(pos uint, array []byte) {
	assert(pos&7 == 0)
	array[pos>>3] = 0
}
