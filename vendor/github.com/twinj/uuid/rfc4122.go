package uuid

/***************
 * Date: 14/02/14
 * Time: 7:44 PM
 ***************/

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"log"
	seed "math/rand"
	"net"
)

const (
	length = 16

	// 3F used by RFC4122 although 1F works for all
	variantSet = 0x3F

	// rather than using 0xC0 we use 0xE0 to retrieve the variant
	// The result is the same for all other variants
	// 0x80 and 0xA0 are used to identify RFC4122 compliance
	variantGet = 0xE0
)

var (
	// nodeID is the default Namespace node
	nodeId = []byte{
		// 00.192.79.212.48.200
		0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
	}
	// The following standard UUIDs are for use with V3 or V5 UUIDs.
	NamespaceDNS  = &Struct{0x6ba7b810, 0x9dad, 0x11d1, 0x80, 0xb4, nodeId, length}
	NamespaceURL  = &Struct{0x6ba7b811, 0x9dad, 0x11d1, 0x80, 0xb4, nodeId, length}
	NamespaceOID  = &Struct{0x6ba7b812, 0x9dad, 0x11d1, 0x80, 0xb4, nodeId, length}
	NamespaceX500 = &Struct{0x6ba7b814, 0x9dad, 0x11d1, 0x80, 0xb4, nodeId, length}

	state State
)

func init() {
	seed.Seed((int64(timestamp())^int64(gregorianToUNIXOffset))*0x6ba7b814<<0x6ba7b812 | 1391463463)
	state = State{
		randomNode:     true,
		randomSequence: true,
		past:           Timestamp((1391463463 * 10000000) + (100 * 10) + gregorianToUNIXOffset),
		node:           nodeId,
		sequence:       uint16(seed.Int()) & 0x3FFF,
		saver:          nil,
	}
}

// NewV1 will generate a new RFC4122 version 1 UUID
func NewV1() UUID {
	state.Lock()
	defer state.Unlock()
	now := currentUUIDTimestamp()
	state.read(now, currentUUIDNodeId())
	state.persist()
	return formatV1(now, uint16(1), ReservedRFC4122, state.node)
}

// NewV3 will generate a new RFC4122 version 3 UUID
// V3 is based on the MD5 hash of a namespace identifier UUID and
// any type which implements the UniqueName interface for the name.
// For strings and slices cast to a Name type
func NewV3(pNs UUID, pName UniqueName) UUID {
	o := new(Array)
	// Set all bits to MD5 hash generated from namespace and name.
	Digest(o, pNs, pName, md5.New())
	o.setRFC4122Variant()
	o.setVersion(3)
	return o
}

// NewV4 will generate a new RFC4122 version 4 UUID
// A cryptographically secure random UUID.
func NewV4() UUID {
	o := new(Array)
	// Read random values (or pseudo-randomly) into Array type.
	_, err := rand.Read(o[:length])
	if err != nil {
		panic(err)
	}
	o.setRFC4122Variant()
	o.setVersion(4)
	return o
}

// NewV5 will generate a new RFC4122 version 5 UUID
// Generate a UUID based on the SHA-1 hash of a namespace
// identifier and a name.
func NewV5(pNs UUID, pName UniqueName) UUID {
	o := new(Array)
	Digest(o, pNs, pName, sha1.New())
	o.setRFC4122Variant()
	o.setVersion(5)
	return o
}

// either generates a random node when there is an error or gets
// the pre initialised one
func currentUUIDNodeId() (node net.HardwareAddr) {
	if state.randomNode {
		b := make([]byte, 16+6)
		_, err := rand.Read(b)
		if err != nil {
			log.Println("UUID.currentUUIDNodeId error:", err)
			node = nodeId
			return
		}
		h := sha1.New()
		h.Write(b)
		binary.Write(h, binary.LittleEndian, state.sequence)
		node = h.Sum(nil)[:6]
		if err != nil {
			log.Println("UUID.currentUUIDNodeId error:", err)
			node = nodeId
			return
		}
		// Mark as randomly generated
		node[0] |= 0x01
	} else {
		node = state.node
	}
	return
}

// Unmarshal data into struct for V1 UUIDs
func formatV1(pNow Timestamp, pVersion uint16, pVariant byte, pNode []byte) UUID {
	o := new(Struct)
	o.timeLow = uint32(pNow & 0xFFFFFFFF)
	o.timeMid = uint16((pNow >> 32) & 0xFFFF)
	o.timeHiAndVersion = uint16((pNow >> 48) & 0x0FFF)
	o.timeHiAndVersion |= uint16(pVersion << 12)
	o.sequenceLow = byte(state.sequence & 0xFF)
	o.sequenceHiAndVariant = byte((state.sequence & 0x3F00) >> 8)
	o.sequenceHiAndVariant |= pVariant
	o.node = pNode
	o.size = length
	return o
}
