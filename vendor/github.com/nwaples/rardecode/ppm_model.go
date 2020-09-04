package rardecode

import (
	"errors"
	"io"
)

const (
	rangeBottom = 1 << 15
	rangeTop    = 1 << 24

	maxFreq = 124

	intBits    = 7
	periodBits = 7
	binScale   = 1 << (intBits + periodBits)

	n0       = 1
	n1       = 4
	n2       = 4
	n3       = 4
	n4       = (128 + 3 - 1*n1 - 2*n2 - 3*n3) / 4
	nIndexes = n0 + n1 + n2 + n3 + n4

	// memory is allocated in units. A unit contains unitSize number of bytes.
	// A unit can store one context or two states.
	unitSize = 12

	maxUint16 = 1<<16 - 1
	freeMark  = -1
)

var (
	errCorruptPPM = errors.New("rardecode: corrupt ppm data")

	expEscape  = []byte{25, 14, 9, 7, 5, 5, 4, 4, 4, 3, 3, 3, 2, 2, 2, 2}
	initBinEsc = []uint16{0x3CDD, 0x1F3F, 0x59BF, 0x48F3, 0x64A1, 0x5ABC, 0x6632, 0x6051}

	ns2Index   [256]byte
	ns2BSIndex [256]byte

	// units2Index maps the number of units in a block to a freelist index
	units2Index [128 + 1]byte
	// index2Units maps a freelist index to the size of the block in units
	index2Units [nIndexes]int32
)

func init() {
	ns2BSIndex[0] = 2 * 0
	ns2BSIndex[1] = 2 * 1
	for i := 2; i < 11; i++ {
		ns2BSIndex[i] = 2 * 2
	}
	for i := 11; i < 256; i++ {
		ns2BSIndex[i] = 2 * 3
	}

	var j, n byte
	for i := range ns2Index {
		ns2Index[i] = n
		if j <= 3 {
			n++
			j = n
		} else {
			j--
		}
	}

	var ii byte
	var iu, units int32
	for i, n := range []int{n0, n1, n2, n3, n4} {
		for j := 0; j < n; j++ {
			units += int32(i)
			index2Units[ii] = units
			for iu <= units {
				units2Index[iu] = ii
				iu++
			}
			ii++
		}
	}
}

type rangeCoder struct {
	br   io.ByteReader
	code uint32
	low  uint32
	rnge uint32
}

func (r *rangeCoder) init(br io.ByteReader) error {
	r.br = br
	r.low = 0
	r.rnge = ^uint32(0)
	for i := 0; i < 4; i++ {
		c, err := r.br.ReadByte()
		if err != nil {
			return err
		}
		r.code = r.code<<8 | uint32(c)
	}
	return nil
}

func (r *rangeCoder) currentCount(scale uint32) uint32 {
	r.rnge /= scale
	return (r.code - r.low) / r.rnge
}

func (r *rangeCoder) normalize() error {
	for {
		if r.low^(r.low+r.rnge) >= rangeTop {
			if r.rnge >= rangeBottom {
				return nil
			}
			r.rnge = -r.low & (rangeBottom - 1)
		}
		c, err := r.br.ReadByte()
		if err != nil {
			return err
		}
		r.code = r.code<<8 | uint32(c)
		r.rnge <<= 8
		r.low <<= 8
	}
}

func (r *rangeCoder) decode(lowCount, highCount uint32) error {
	r.low += r.rnge * lowCount
	r.rnge *= highCount - lowCount

	return r.normalize()
}

type see2Context struct {
	summ  uint16
	shift byte
	count byte
}

func newSee2Context(i uint16) see2Context {
	return see2Context{i << (periodBits - 4), (periodBits - 4), 4}
}

func (s *see2Context) mean() uint32 {
	if s == nil {
		return 1
	}
	n := s.summ >> s.shift
	if n == 0 {
		return 1
	}
	s.summ -= n
	return uint32(n)
}

func (s *see2Context) update() {
	if s == nil || s.shift >= periodBits {
		return
	}
	s.count--
	if s.count == 0 {
		s.summ += s.summ
		s.count = 3 << s.shift
		s.shift++
	}
}

type state struct {
	sym  byte
	freq byte

	// succ can point to a context or byte in memory.
	// A context pointer is a positive integer. It is an index into the states
	// array that points to the first of two states which the context is
	// marshalled into.
	// A byte pointer is a negative integer. The magnitude represents the position
	// in bytes from the bottom of the memory. As memory is modelled as an array of
	// states, this is used to calculate which state, and where in the state the
	// byte is stored.
	// A zero value represents a nil pointer.
	succ int32
}

// uint16 return a uint16 stored in the sym and freq fields of a state
func (s state) uint16() uint16 { return uint16(s.sym) | uint16(s.freq)<<8 }

// setUint16 stores a uint16 in the sym and freq fields of a state
func (s *state) setUint16(n uint16) { s.sym = byte(n); s.freq = byte(n >> 8) }

// A context is marshalled into a slice of two states.
// The first state contains the number of states, and the suffix pointer.
// If there is only one state, the second state contains that state.
// If there is more than one state, the second state contains the summFreq
// and the index to the slice of states.
type context struct {
	i int32   // index into the states array for context
	s []state // slice of two states representing context
	a *subAllocator
}

// succPtr returns a pointer value for the context to be stored in a state.succ
func (c *context) succPtr() int32 { return c.i }

func (c *context) numStates() int { return int(c.s[0].uint16()) }

func (c *context) setNumStates(n int) { c.s[0].setUint16(uint16(n)) }

func (c *context) statesIndex() int32 { return c.s[1].succ }

func (c *context) setStatesIndex(n int32) { c.s[1].succ = n }

func (c *context) suffix() *context { return c.a.succContext(c.s[0].succ) }

func (c *context) setSuffix(sc *context) { c.s[0].succ = sc.i }

func (c *context) summFreq() uint16 { return c.s[1].uint16() }

func (c *context) setSummFreq(f uint16) { c.s[1].setUint16(f) }

func (c *context) notEq(ctx *context) bool { return c.i != ctx.i }

func (c *context) states() []state {
	if ns := int32(c.s[0].uint16()); ns != 1 {
		i := c.s[1].succ
		return c.a.states[i : i+ns]
	}
	return c.s[1:]
}

// shrinkStates shrinks the state list down to size states
func (c *context) shrinkStates(states []state, size int) []state {
	i1 := units2Index[(len(states)+1)>>1]
	i2 := units2Index[(size+1)>>1]

	if size == 1 {
		// store state in context, and free states block
		n := c.statesIndex()
		c.s[1] = states[0]
		states = c.s[1:]
		c.a.addFreeBlock(n, i1)
	} else if i1 != i2 {
		if n := c.a.removeFreeBlock(i2); n > 0 {
			// allocate new block and copy
			copy(c.a.states[n:], states[:size])
			states = c.a.states[n:]
			// free old block
			c.a.addFreeBlock(c.statesIndex(), i1)
			c.setStatesIndex(n)
		} else {
			// split current block, and free units not needed
			n = c.statesIndex() + index2Units[i2]<<1
			u := index2Units[i1] - index2Units[i2]
			c.a.freeUnits(n, u)
		}
	}
	c.setNumStates(size)
	return states[:size]
}

// expandStates expands the states list by one
func (c *context) expandStates() []state {
	states := c.states()
	ns := len(states)
	if ns == 1 {
		s := states[0]
		n := c.a.allocUnits(1)
		if n == 0 {
			return nil
		}
		c.setStatesIndex(n)
		states = c.a.states[n:]
		states[0] = s
	} else if ns&0x1 == 0 {
		u := ns >> 1
		i1 := units2Index[u]
		i2 := units2Index[u+1]
		if i1 != i2 {
			n := c.a.allocUnits(i2)
			if n == 0 {
				return nil
			}
			copy(c.a.states[n:], states)
			c.a.addFreeBlock(c.statesIndex(), i1)
			c.setStatesIndex(n)
			states = c.a.states[n:]
		}
	}
	c.setNumStates(ns + 1)
	return states[:ns+1]
}

type subAllocator struct {
	// memory for allocation is split into two heaps

	heap1MaxBytes int32 // maximum bytes available in heap1
	heap1Lo       int32 // heap1 bottom in number of bytes
	heap1Hi       int32 // heap1 top in number of bytes
	heap2Lo       int32 // heap2 bottom index in states
	heap2Hi       int32 // heap2 top index in states
	glueCount     int

	// Each freeList entry contains an index into states for the beginning
	// of a free block. The first state in that block may contain an index
	// to another free block and so on. The size of the free block in units
	// (2 states) for that freeList index can be determined from the
	// index2Units array.
	freeList [nIndexes]int32

	// Instead of bytes, memory is represented by a slice of states.
	// context's are marshalled to and from a pair of states.
	// multiple bytes are stored in a state.
	states []state
}

func (a *subAllocator) init(maxMB int) {
	bytes := int32(maxMB) << 20
	heap2Units := bytes / 8 / unitSize * 7
	a.heap1MaxBytes = bytes - heap2Units*unitSize
	// Add one for the case when bytes are not a multiple of unitSize
	heap1Units := a.heap1MaxBytes/unitSize + 1
	// Calculate total size in state's. Add 1 unit so we can reserve the first unit.
	// This will allow us to use the zero index as a nil pointer.
	n := int(1+heap1Units+heap2Units) * 2
	if cap(a.states) > n {
		a.states = a.states[:n]
	} else {
		a.states = make([]state, n)
	}
}

func (a *subAllocator) restart() {
	// Pad heap1 start by 1 unit and enough bytes so that there is no
	// gap between heap1 end and heap2 start.
	a.heap1Lo = unitSize + (unitSize - a.heap1MaxBytes%unitSize)
	a.heap1Hi = unitSize + (a.heap1MaxBytes/unitSize+1)*unitSize
	a.heap2Lo = a.heap1Hi / unitSize * 2
	a.heap2Hi = int32(len(a.states))
	a.glueCount = 0
	for i := range a.freeList {
		a.freeList[i] = 0
	}
	for i := range a.states {
		a.states[i] = state{}
	}
}

// pushByte puts a byte on the heap and returns a state.succ index that
// can be used to retrieve it.
func (a *subAllocator) pushByte(c byte) int32 {
	si := a.heap1Lo / 6 // state index
	oi := a.heap1Lo % 6 // byte position in state
	switch oi {
	case 0:
		a.states[si].sym = c
	case 1:
		a.states[si].freq = c
	default:
		n := (uint(oi) - 2) * 8
		mask := ^(uint32(0xFF) << n)
		succ := uint32(a.states[si].succ) & mask
		succ |= uint32(c) << n
		a.states[si].succ = int32(succ)
	}
	a.heap1Lo++
	if a.heap1Lo >= a.heap1Hi {
		return 0
	}
	return -a.heap1Lo
}

// popByte reverses the previous pushByte
func (a *subAllocator) popByte() { a.heap1Lo-- }

// succByte returns a byte from the heap given a state.succ index
func (a *subAllocator) succByte(i int32) byte {
	i = -i
	si := i / 6
	oi := i % 6
	switch oi {
	case 0:
		return a.states[si].sym
	case 1:
		return a.states[si].freq
	default:
		n := (uint(oi) - 2) * 8
		succ := uint32(a.states[si].succ) >> n
		return byte(succ & 0xff)
	}
}

// succContext returns a context given a state.succ index
func (a *subAllocator) succContext(i int32) *context {
	if i <= 0 {
		return nil
	}
	return &context{i: i, s: a.states[i : i+2 : i+2], a: a}
}

// succIsNil returns whether a state.succ points to nothing
func (a *subAllocator) succIsNil(i int32) bool { return i == 0 }

// nextByteAddr takes a state.succ value representing a pointer
// to a byte, and returns the next bytes address
func (a *subAllocator) nextByteAddr(n int32) int32 { return n - 1 }

func (a *subAllocator) removeFreeBlock(i byte) int32 {
	n := a.freeList[i]
	if n != 0 {
		a.freeList[i] = a.states[n].succ
		a.states[n] = state{}
	}
	return n
}

func (a *subAllocator) addFreeBlock(n int32, i byte) {
	a.states[n].succ = a.freeList[i]
	a.freeList[i] = n
}

func (a *subAllocator) freeUnits(n, u int32) {
	i := units2Index[u]
	if u != index2Units[i] {
		i--
		a.addFreeBlock(n, i)
		u -= index2Units[i]
		n += index2Units[i] << 1
		i = units2Index[u]
	}
	a.addFreeBlock(n, i)
}

func (a *subAllocator) glueFreeBlocks() {
	var freeIndex int32

	for i, n := range a.freeList {
		s := state{succ: freeMark}
		s.setUint16(uint16(index2Units[i]))
		for n != 0 {
			states := a.states[n:]
			states[1].succ = freeIndex
			freeIndex = n
			n = states[0].succ
			states[0] = s
		}
		a.freeList[i] = 0
	}

	for i := freeIndex; i != 0; i = a.states[i+1].succ {
		if a.states[i].succ != freeMark {
			continue
		}
		u := int32(a.states[i].uint16())
		states := a.states[i+u<<1:]
		for len(states) > 0 && states[0].succ == freeMark {
			u += int32(states[0].uint16())
			if u > maxUint16 {
				break
			}
			states[0].succ = 0
			a.states[i].setUint16(uint16(u))
			states = a.states[i+u<<1:]
		}
	}

	for n := freeIndex; n != 0; n = a.states[n+1].succ {
		if a.states[n].succ != freeMark {
			continue
		}
		a.states[n].succ = 0
		u := int32(a.states[n].uint16())
		m := n
		for u > 128 {
			a.addFreeBlock(m, nIndexes-1)
			u -= 128
			m += 256
		}
		a.freeUnits(m, u)
	}
}

func (a *subAllocator) allocUnitsRare(index byte) int32 {
	if a.glueCount == 0 {
		a.glueCount = 255
		a.glueFreeBlocks()
		if n := a.removeFreeBlock(index); n > 0 {
			return n
		}
	}
	// try to find a larger free block and split it
	for i := index + 1; i < nIndexes; i++ {
		if n := a.removeFreeBlock(i); n > 0 {
			u := index2Units[i] - index2Units[index]
			a.freeUnits(n+index2Units[index]<<1, u)
			return n
		}
	}
	a.glueCount--

	// try to allocate units from the top of heap1
	n := a.heap1Hi - index2Units[index]*unitSize
	if n > a.heap1Lo {
		a.heap1Hi = n
		return a.heap1Hi / unitSize * 2
	}
	return 0
}

func (a *subAllocator) allocUnits(i byte) int32 {
	// try to allocate a free block
	if n := a.removeFreeBlock(i); n > 0 {
		return n
	}
	// try to allocate from the bottom of heap2
	n := index2Units[i] << 1
	if a.heap2Lo+n <= a.heap2Hi {
		lo := a.heap2Lo
		a.heap2Lo += n
		return lo
	}
	return a.allocUnitsRare(i)
}

func (a *subAllocator) newContext(s state, suffix *context) *context {
	var n int32
	if a.heap2Lo < a.heap2Hi {
		// allocate from top of heap2
		a.heap2Hi -= 2
		n = a.heap2Hi
	} else if n = a.removeFreeBlock(1); n == 0 {
		if n = a.allocUnitsRare(1); n == 0 {
			return nil
		}
	}
	c := &context{i: n, s: a.states[n : n+2 : n+2], a: a}
	c.s[0] = state{}
	c.setNumStates(1)
	c.s[1] = s
	if suffix != nil {
		c.setSuffix(suffix)
	}
	return c
}

func (a *subAllocator) newContextSize(ns int) *context {
	c := a.newContext(state{}, nil)
	c.setNumStates(ns)
	i := units2Index[(ns+1)>>1]
	n := a.allocUnits(i)
	c.setStatesIndex(n)
	return c
}

type model struct {
	maxOrder    int
	orderFall   int
	initRL      int
	runLength   int
	prevSuccess byte
	escCount    byte
	prevSym     byte
	initEsc     byte
	minC        *context
	maxC        *context
	rc          rangeCoder
	a           subAllocator
	charMask    [256]byte
	binSumm     [128][64]uint16
	see2Cont    [25][16]see2Context
}

func (m *model) restart() {
	for i := range m.charMask {
		m.charMask[i] = 0
	}
	m.escCount = 1

	if m.maxOrder < 12 {
		m.initRL = -m.maxOrder - 1
	} else {
		m.initRL = -12 - 1
	}
	m.orderFall = m.maxOrder
	m.runLength = m.initRL
	m.prevSuccess = 0

	m.a.restart()

	c := m.a.newContextSize(256)
	c.setSummFreq(257)
	states := c.states()
	for i := range states {
		states[i] = state{sym: byte(i), freq: 1}
	}
	m.minC = c
	m.maxC = c
	m.prevSym = 0

	for i := range m.binSumm {
		for j, esc := range initBinEsc {
			n := binScale - esc/(uint16(i)+2)
			for k := j; k < len(m.binSumm[i]); k += len(initBinEsc) {
				m.binSumm[i][k] = n
			}
		}
	}

	for i := range m.see2Cont {
		see := newSee2Context(5*uint16(i) + 10)
		for j := range m.see2Cont[i] {
			m.see2Cont[i][j] = see
		}
	}
}

func (m *model) init(br io.ByteReader, reset bool, maxOrder, maxMB int) error {
	err := m.rc.init(br)
	if err != nil {
		return err
	}
	if !reset {
		if m.minC == nil {
			return errCorruptPPM
		}
		return nil
	}

	m.a.init(maxMB)

	if maxOrder == 1 {
		return errCorruptPPM
	}
	m.maxOrder = maxOrder
	m.restart()
	return nil
}

func (m *model) rescale(s *state) *state {
	if s.freq <= maxFreq {
		return s
	}
	c := m.minC

	var summFreq uint16

	s.freq += 4
	states := c.states()
	escFreq := c.summFreq() + 4

	for i := range states {
		f := states[i].freq
		escFreq -= uint16(f)
		if m.orderFall != 0 {
			f++
		}
		f >>= 1
		summFreq += uint16(f)
		states[i].freq = f

		if i == 0 || f <= states[i-1].freq {
			continue
		}
		j := i - 1
		for j > 0 && f > states[j-1].freq {
			j--
		}
		t := states[i]
		copy(states[j+1:i+1], states[j:i])
		states[j] = t
	}

	i := len(states) - 1
	for states[i].freq == 0 {
		i--
		escFreq++
	}
	if i != len(states)-1 {
		states = c.shrinkStates(states, i+1)
	}
	s = &states[0]
	if i == 0 {
		for {
			s.freq -= s.freq >> 1
			escFreq >>= 1
			if escFreq <= 1 {
				return s
			}
		}
	}
	summFreq += escFreq - (escFreq >> 1)
	c.setSummFreq(summFreq)
	return s
}

func (m *model) decodeBinSymbol() (*state, error) {
	c := m.minC
	s := &c.states()[0]

	ns := c.suffix().numStates()
	i := m.prevSuccess + ns2BSIndex[ns-1] + byte(m.runLength>>26)&0x20
	if m.prevSym >= 64 {
		i += 8
	}
	if s.sym >= 64 {
		i += 2 * 8
	}
	bs := &m.binSumm[s.freq-1][i]
	mean := (*bs + 1<<(periodBits-2)) >> periodBits

	if m.rc.currentCount(binScale) < uint32(*bs) {
		err := m.rc.decode(0, uint32(*bs))
		if s.freq < 128 {
			s.freq++
		}
		*bs += 1<<intBits - mean
		m.prevSuccess = 1
		m.runLength++
		return s, err
	}
	err := m.rc.decode(uint32(*bs), binScale)
	*bs -= mean
	m.initEsc = expEscape[*bs>>10]
	m.charMask[s.sym] = m.escCount
	m.prevSuccess = 0
	return nil, err
}

func (m *model) decodeSymbol1() (*state, error) {
	c := m.minC
	states := c.states()
	scale := uint32(c.summFreq())
	// protect against divide by zero
	// TODO: look at why this happens, may be problem elsewhere
	if scale == 0 {
		return nil, errCorruptPPM
	}
	count := m.rc.currentCount(scale)
	m.prevSuccess = 0

	var n uint32
	for i := range states {
		s := &states[i]
		n += uint32(s.freq)
		if n <= count {
			continue
		}
		err := m.rc.decode(n-uint32(s.freq), n)
		s.freq += 4
		c.setSummFreq(uint16(scale + 4))
		if i == 0 {
			if 2*n > scale {
				m.prevSuccess = 1
				m.runLength++
			}
		} else {
			if s.freq <= states[i-1].freq {
				return s, err
			}
			states[i-1], states[i] = states[i], states[i-1]
			s = &states[i-1]
		}
		return m.rescale(s), err
	}

	for _, s := range states {
		m.charMask[s.sym] = m.escCount
	}
	return nil, m.rc.decode(n, scale)
}

func (m *model) makeEscFreq(c *context, numMasked int) *see2Context {
	ns := c.numStates()
	if ns == 256 {
		return nil
	}
	diff := ns - numMasked

	var i int
	if m.prevSym >= 64 {
		i = 8
	}
	if diff < c.suffix().numStates()-ns {
		i++
	}
	if int(c.summFreq()) < 11*ns {
		i += 2
	}
	if numMasked > diff {
		i += 4
	}
	return &m.see2Cont[ns2Index[diff-1]][i]
}

func (m *model) decodeSymbol2(numMasked int) (*state, error) {
	c := m.minC

	see := m.makeEscFreq(c, numMasked)
	scale := see.mean()

	var i int
	var hi uint32
	states := c.states()
	sl := make([]*state, len(states)-numMasked)
	for j := range sl {
		for m.charMask[states[i].sym] == m.escCount {
			i++
		}
		hi += uint32(states[i].freq)
		sl[j] = &states[i]
		i++
	}

	scale += hi
	count := m.rc.currentCount(scale)

	if count >= scale {
		return nil, errCorruptPPM
	}
	if count >= hi {
		err := m.rc.decode(hi, scale)
		if see != nil {
			see.summ += uint16(scale)
		}
		for _, s := range sl {
			m.charMask[s.sym] = m.escCount
		}
		return nil, err
	}

	hi = uint32(sl[0].freq)
	for hi <= count {
		sl = sl[1:]
		hi += uint32(sl[0].freq)
	}
	s := sl[0]

	err := m.rc.decode(hi-uint32(s.freq), hi)

	see.update()

	m.escCount++
	m.runLength = m.initRL

	s.freq += 4
	c.setSummFreq(c.summFreq() + 4)
	return m.rescale(s), err
}

func (c *context) findState(sym byte) *state {
	var i int
	states := c.states()
	for i = range states {
		if states[i].sym == sym {
			break
		}
	}
	return &states[i]
}

func (m *model) createSuccessors(s, ss *state) *context {
	var sl []*state

	if m.orderFall != 0 {
		sl = append(sl, s)
	}

	c := m.minC
	for suff := c.suffix(); suff != nil; suff = c.suffix() {
		c = suff

		if ss == nil {
			ss = c.findState(s.sym)
		}
		if ss.succ != s.succ {
			c = m.a.succContext(ss.succ)
			break
		}
		sl = append(sl, ss)
		ss = nil
	}

	if len(sl) == 0 {
		return c
	}

	var up state
	up.sym = m.a.succByte(s.succ)
	up.succ = m.a.nextByteAddr(s.succ)

	states := c.states()
	if len(states) > 1 {
		s = c.findState(up.sym)

		cf := uint16(s.freq) - 1
		s0 := c.summFreq() - uint16(len(states)) - cf

		if 2*cf <= s0 {
			if 5*cf > s0 {
				up.freq = 2
			} else {
				up.freq = 1
			}
		} else {
			up.freq = byte(1 + (2*cf+3*s0-1)/(2*s0))
		}
	} else {
		up.freq = states[0].freq
	}

	for i := len(sl) - 1; i >= 0; i-- {
		c = m.a.newContext(up, c)
		if c == nil {
			return nil
		}
		sl[i].succ = c.succPtr()
	}
	return c
}

func (m *model) update(s *state) {
	if m.orderFall == 0 {
		if c := m.a.succContext(s.succ); c != nil {
			m.minC = c
			m.maxC = c
			return
		}
	}

	if m.escCount == 0 {
		m.escCount = 1
		for i := range m.charMask {
			m.charMask[i] = 0
		}
	}

	var ss *state // matching minC.suffix state

	if s.freq < maxFreq/4 && m.minC.suffix() != nil {
		c := m.minC.suffix()
		states := c.states()

		var i int
		if len(states) > 1 {
			for states[i].sym != s.sym {
				i++
			}
			if i > 0 && states[i].freq >= states[i-1].freq {
				states[i-1], states[i] = states[i], states[i-1]
				i--
			}
			if states[i].freq < maxFreq-9 {
				states[i].freq += 2
				c.setSummFreq(c.summFreq() + 2)
			}
		} else if states[0].freq < 32 {
			states[0].freq++
		}
		ss = &states[i] // save later for createSuccessors
	}

	if m.orderFall == 0 {
		c := m.createSuccessors(s, ss)
		if c == nil {
			m.restart()
		} else {
			m.minC = c
			m.maxC = c
			s.succ = c.succPtr()
		}
		return
	}

	succ := m.a.pushByte(s.sym)
	if m.a.succIsNil(succ) {
		m.restart()
		return
	}

	var minC *context
	if m.a.succIsNil(s.succ) {
		s.succ = succ
		minC = m.minC
	} else {
		minC = m.a.succContext(s.succ)
		if minC == nil {
			minC = m.createSuccessors(s, ss)
			if minC == nil {
				m.restart()
				return
			}
		}
		m.orderFall--
		if m.orderFall == 0 {
			succ = minC.succPtr()
			if m.maxC.notEq(m.minC) {
				m.a.popByte()
			}
		}
	}

	n := m.minC.numStates()
	s0 := int(m.minC.summFreq()) - n - int(s.freq-1)
	for c := m.maxC; c.notEq(m.minC); c = c.suffix() {
		var summFreq uint16

		states := c.expandStates()
		if states == nil {
			m.restart()
			return
		}
		if ns := len(states) - 1; ns != 1 {
			summFreq = c.summFreq()
			if 4*ns <= n && int(summFreq) <= 8*ns {
				summFreq += 2
			}
			if 2*ns < n {
				summFreq++
			}
		} else {
			p := &states[0]
			if p.freq < maxFreq/4-1 {
				p.freq += p.freq
			} else {
				p.freq = maxFreq - 4
			}
			summFreq = uint16(p.freq) + uint16(m.initEsc)
			if n > 3 {
				summFreq++
			}
		}

		cf := 2 * int(s.freq) * int(summFreq+6)
		sf := s0 + int(summFreq)
		var freq byte
		if cf >= 6*sf {
			switch {
			case cf >= 15*sf:
				freq = 7
			case cf >= 12*sf:
				freq = 6
			case cf >= 9*sf:
				freq = 5
			default:
				freq = 4
			}
			summFreq += uint16(freq)
		} else {
			switch {
			case cf >= 4*sf:
				freq = 3
			case cf > sf:
				freq = 2
			default:
				freq = 1
			}
			summFreq += 3
		}
		states[len(states)-1] = state{sym: s.sym, freq: freq, succ: succ}
		c.setSummFreq(summFreq)
	}
	m.minC = minC
	m.maxC = minC
}

func (m *model) ReadByte() (byte, error) {
	if m.minC == nil {
		return 0, errCorruptPPM
	}
	var s *state
	var err error
	if m.minC.numStates() == 1 {
		s, err = m.decodeBinSymbol()
	} else {
		s, err = m.decodeSymbol1()
	}
	for s == nil && err == nil {
		n := m.minC.numStates()
		for m.minC.numStates() == n {
			m.orderFall++
			m.minC = m.minC.suffix()
			if m.minC == nil {
				return 0, errCorruptPPM
			}
		}
		s, err = m.decodeSymbol2(n)
	}
	if err != nil {
		return 0, err
	}

	// save sym so it doesn't get overwritten by a possible restart()
	sym := s.sym
	m.update(s)
	m.prevSym = sym
	return sym, nil
}
