package hbase

import "bytes"

type Type byte

const (
	TypeMinimum             = Type(0)
	TypePut                 = Type(4)
	TypeDelete              = Type(8)
	TypeDeleteFamilyVersion = Type(10)
	TypeDeleteColumn        = Type(12)
	TypeDeleteFamily        = Type(14)
	TypeMaximum             = Type(0xff)
)

type set map[string]struct{}

func newSet() set {
	return set(map[string]struct{}{})
}

func (s set) exists(k string) bool {
	_, ok := s[k]
	return ok
}

func (s set) add(k string) {
	s[k] = struct{}{}
}

func (s set) remove(k string) {
	delete(s, k)
}

type BytesSlice [][]byte

func (s BytesSlice) Len() int           { return len(s) }
func (s BytesSlice) Less(i, j int) bool { return bytes.Compare(s[i], s[j]) < 0 }
func (s BytesSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
