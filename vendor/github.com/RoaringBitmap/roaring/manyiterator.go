package roaring

type manyIterable interface {
	nextMany(hs uint32, buf []uint32) int
}

type manyIterator struct {
	slice []uint16
	loc   int
}

func (si *manyIterator) nextMany(hs uint32, buf []uint32) int {
	n := 0
	l := si.loc
	s := si.slice
	for n < len(buf) && l < len(s) {
		buf[n] = uint32(s[l]) | hs
		l++
		n++
	}
	si.loc = l
	return n
}
