// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package table allows read and write sorted key/value.
package table

import (
	"encoding/binary"
)

/*
Table:

Table is consist of one or more data blocks, an optional filter block
a metaindex block, an index block and a table footer. Metaindex block
is a special block used to keep parameters of the table, such as filter
block name and its block handle. Index block is a special block used to
keep record of data blocks offset and length, index block use one as
restart interval. The key used by index block are the last key of preceding
block, shorter separator of adjacent blocks or shorter successor of the
last key of the last block. Filter block is an optional block contains
sequence of filter data generated by a filter generator.

Table data structure:
                                                         + optional
                                                        /
    +--------------+--------------+--------------+------+-------+-----------------+-------------+--------+
    | data block 1 |      ...     | data block n | filter block | metaindex block | index block | footer |
    +--------------+--------------+--------------+--------------+-----------------+-------------+--------+

    Each block followed by a 5-bytes trailer contains compression type and checksum.

Table block trailer:

    +---------------------------+-------------------+
    | compression type (1-byte) | checksum (4-byte) |
    +---------------------------+-------------------+

    The checksum is a CRC-32 computed using Castagnoli's polynomial. Compression
    type also included in the checksum.

Table footer:

      +------------------- 40-bytes -------------------+
     /                                                  \
    +------------------------+--------------------+------+-----------------+
    | metaindex block handle / index block handle / ---- | magic (8-bytes) |
    +------------------------+--------------------+------+-----------------+

    The magic are first 64-bit of SHA-1 sum of "http://code.google.com/p/leveldb/".

NOTE: All fixed-length integer are little-endian.
*/

/*
Block:

Block is consist of one or more key/value entries and a block trailer.
Block entry shares key prefix with its preceding key until a restart
point reached. A block should contains at least one restart point.
First restart point are always zero.

Block data structure:

      + restart point                 + restart point (depends on restart interval)
     /                               /
    +---------------+---------------+---------------+---------------+---------+
    | block entry 1 | block entry 2 |      ...      | block entry n | trailer |
    +---------------+---------------+---------------+---------------+---------+

Key/value entry:

              +---- key len ----+
             /                   \
    +-------+---------+-----------+---------+--------------------+--------------+----------------+
    | shared (varint) | not shared (varint) | value len (varint) | key (varlen) | value (varlen) |
    +-----------------+---------------------+--------------------+--------------+----------------+

    Block entry shares key prefix with its preceding key:
    Conditions:
        restart_interval=2
        entry one  : key=deck,value=v1
        entry two  : key=dock,value=v2
        entry three: key=duck,value=v3
    The entries will be encoded as follow:

      + restart point (offset=0)                                                 + restart point (offset=16)
     /                                                                          /
    +-----+-----+-----+----------+--------+-----+-----+-----+---------+--------+-----+-----+-----+----------+--------+
    |  0  |  4  |  2  |  "deck"  |  "v1"  |  1  |  3  |  2  |  "ock"  |  "v2"  |  0  |  4  |  2  |  "duck"  |  "v3"  |
    +-----+-----+-----+----------+--------+-----+-----+-----+---------+--------+-----+-----+-----+----------+--------+
     \                                   / \                                  / \                                   /
      +----------- entry one -----------+   +----------- entry two ----------+   +---------- entry three ----------+

    The block trailer will contains two restart points:

    +------------+-----------+--------+
    |     0      |    16     |   2    |
    +------------+-----------+---+----+
     \                      /     \
      +-- restart points --+       + restart points length

Block trailer:

      +-- 4-bytes --+
     /               \
    +-----------------+-----------------+-----------------+------------------------------+
    | restart point 1 |       ....      | restart point n | restart points len (4-bytes) |
    +-----------------+-----------------+-----------------+------------------------------+


NOTE: All fixed-length integer are little-endian.
*/

/*
Filter block:

Filter block consist of one or more filter data and a filter block trailer.
The trailer contains filter data offsets, a trailer offset and a 1-byte base Lg.

Filter block data structure:

      + offset 1      + offset 2      + offset n      + trailer offset
     /               /               /               /
    +---------------+---------------+---------------+---------+
    | filter data 1 |      ...      | filter data n | trailer |
    +---------------+---------------+---------------+---------+

Filter block trailer:

      +- 4-bytes -+
     /             \
    +---------------+---------------+---------------+-------------------------------+------------------+
    | data 1 offset |      ....     | data n offset | data-offsets offset (4-bytes) | base Lg (1-byte) |
    +-------------- +---------------+---------------+-------------------------------+------------------+


NOTE: All fixed-length integer are little-endian.
*/

const (
	blockTrailerLen = 5
	footerLen       = 48

	magic = "\x57\xfb\x80\x8b\x24\x75\x47\xdb"

	// The block type gives the per-block compression format.
	// These constants are part of the file format and should not be changed.
	blockTypeNoCompression     = 0
	blockTypeSnappyCompression = 1

	// Generate new filter every 2KB of data
	filterBaseLg = 11
	filterBase   = 1 << filterBaseLg
)

type blockHandle struct {
	offset, length uint64
}

func decodeBlockHandle(src []byte) (blockHandle, int) {
	offset, n := binary.Uvarint(src)
	length, m := binary.Uvarint(src[n:])
	if n == 0 || m == 0 {
		return blockHandle{}, 0
	}
	return blockHandle{offset, length}, n + m
}

func encodeBlockHandle(dst []byte, b blockHandle) int {
	n := binary.PutUvarint(dst, b.offset)
	m := binary.PutUvarint(dst[n:], b.length)
	return n + m
}
