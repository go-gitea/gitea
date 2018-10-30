//  Copyright (c) 2017 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package levenshtein

import (
	"encoding/binary"
	"fmt"
	"unicode"

	unicode_utf8 "unicode/utf8"

	"github.com/couchbase/vellum/utf8"
)

var sequences0ToMaxRune utf8.Sequences

func init() {
	sequences0ToMaxRune, _ = utf8.NewSequences(0, unicode.MaxRune)
}

type dfa struct {
	states statesStack
}

type state struct {
	next  []int
	match bool
}

func (s *state) String() string {
	rv := "  |"
	for i := 0; i < 16; i++ {
		rv += fmt.Sprintf("% 5x", i)
	}
	rv += "\n"
	for i := 0; i < len(s.next); i++ {
		if i%16 == 0 {
			rv += fmt.Sprintf("%x |", i/16)
		}
		if s.next[i] != 0 {
			rv += fmt.Sprintf("% 5d", s.next[i])
		} else {
			rv += "    -"
		}
		if i%16 == 15 {
			rv += "\n"
		}
	}
	return rv
}

type dfaBuilder struct {
	dfa    *dfa
	lev    *dynamicLevenshtein
	cache  map[string]int
	keyBuf []byte

	sequences  utf8.Sequences
	rangeStack utf8.RangeStack
	startBytes []byte
	endBytes   []byte
	nexts      []int
}

func newDfaBuilder(lev *dynamicLevenshtein) *dfaBuilder {
	dfab := &dfaBuilder{
		dfa: &dfa{
			states: make([]state, 0, 16),
		},
		lev:        lev,
		cache:      make(map[string]int, 1024),
		startBytes: make([]byte, unicode_utf8.UTFMax),
		endBytes:   make([]byte, unicode_utf8.UTFMax),
	}
	_, dfab.nexts = dfab.newState(false, nil) // create state 0, invalid
	return dfab
}

func (b *dfaBuilder) build() (*dfa, error) {
	var stack intsStack
	stack = stack.Push(b.lev.start())
	seen := make(map[int]struct{})

	var levState []int
	stack, levState = stack.Pop()
	for levState != nil {
		dfaSi := b.cachedState(levState)
		mmToSi, mmMismatchState, err := b.addMismatchUtf8States(dfaSi, levState)
		if err != nil {
			return nil, err
		}
		if mmToSi != 0 {
			if _, ok := seen[mmToSi]; !ok {
				seen[mmToSi] = struct{}{}
				stack = stack.Push(mmMismatchState)
			}
		}

		i := 0
		for _, r := range b.lev.query {
			if uint(levState[i]) > b.lev.distance {
				i++
				continue
			}
			levNext := b.lev.accept(levState, &r)
			nextSi := b.cachedState(levNext)
			if nextSi != 0 {
				err = b.addUtf8RuneRange(true, dfaSi, nextSi, r, r)
				if err != nil {
					return nil, err
				}
				if _, ok := seen[nextSi]; !ok {
					seen[nextSi] = struct{}{}
					stack = stack.Push(levNext)
				}
			}
			i++
		}

		if len(b.dfa.states) > StateLimit {
			return nil, ErrTooManyStates
		}

		stack, levState = stack.Pop()
	}

	return b.dfa, nil
}

func (b *dfaBuilder) cachedState(levState []int) int {
	rv, _ := b.cached(levState)
	return rv
}

func levStateKey(levState []int, buf []byte) []byte {
	if cap(buf) < 8*len(levState) {
		buf = make([]byte, 8*len(levState))
	} else {
		buf = buf[0 : 8*len(levState)]
	}
	for i, state := range levState {
		binary.LittleEndian.PutUint64(buf[i*8:], uint64(state))
	}
	return buf
}

func (b *dfaBuilder) cached(levState []int) (int, bool) {
	if !b.lev.canMatch(levState) {
		return 0, true
	}
	b.keyBuf = levStateKey(levState, b.keyBuf)
	v, ok := b.cache[string(b.keyBuf)]
	if ok {
		return v, true
	}
	match := b.lev.isMatch(levState)
	b.dfa.states = append(b.dfa.states, state{
		next:  make([]int, 256),
		match: match,
	})
	newV := len(b.dfa.states) - 1
	b.cache[string(b.keyBuf)] = newV
	return newV, false
}

func (b *dfaBuilder) addMismatchUtf8States(fromSi int, levState []int) (int, []int, error) {
	mmState := b.lev.accept(levState, nil)
	toSi, _ := b.cached(mmState)
	if toSi == 0 {
		return 0, nil, nil
	}
	b.addUtf8Sequences(false, fromSi, toSi, sequences0ToMaxRune)
	return toSi, mmState, nil
}

func (b *dfaBuilder) addUtf8RuneRange(overwrite bool, fromSi, toSi int,
	fromChar, toChar rune) (
	err error) {
	b.sequences, b.rangeStack, err = utf8.NewSequencesPrealloc(fromChar, toChar,
		b.sequences, b.rangeStack, b.startBytes, b.endBytes)
	if err != nil {
		return err
	}

	b.addUtf8Sequences(overwrite, fromSi, toSi, b.sequences)

	return nil
}

func (b *dfaBuilder) addUtf8Sequences(overwrite bool, fromSi, toSi int,
	sequences utf8.Sequences) {
	for _, seq := range sequences {
		fsi := fromSi
		for _, utf8r := range seq[:len(seq)-1] {
			var tsi int
			tsi, b.nexts = b.newState(false, b.nexts)
			b.addUtf8Range(overwrite, fsi, tsi, utf8r)
			fsi = tsi
		}
		b.addUtf8Range(overwrite, fsi, toSi, seq[len(seq)-1])
	}
}

func (b *dfaBuilder) addUtf8Range(overwrite bool, from, to int, rang utf8.Range) {
	fromNext := b.dfa.states[from].next
	if overwrite {
		for by := rang.Start; by <= rang.End; by++ {
			fromNext[by] = to
		}
	} else {
		for by := rang.Start; by <= rang.End; by++ {
			if fromNext[by] == 0 {
				fromNext[by] = to
			}
		}
	}
}

func (b *dfaBuilder) newState(match bool, prealloc []int) (int, []int) {
	if len(prealloc) < 256 {
		prealloc = make([]int, 16384)
	}
	next := prealloc[0:256]
	prealloc = prealloc[256:]

	b.dfa.states = append(b.dfa.states, state{
		next:  next,
		match: match,
	})

	return len(b.dfa.states) - 1, prealloc
}
