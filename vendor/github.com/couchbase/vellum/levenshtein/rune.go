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

import "unicode/utf8"

// dynamicLevenshtein is the rune-based automaton, which is used
// during the building of the ut8-aware byte-based automaton
type dynamicLevenshtein struct {
	query    string
	distance uint
}

func (d *dynamicLevenshtein) start() []int {
	runeCount := utf8.RuneCountInString(d.query)
	rv := make([]int, runeCount+1)
	for i := 0; i < runeCount+1; i++ {
		rv[i] = i
	}
	return rv
}

func (d *dynamicLevenshtein) isMatch(state []int) bool {
	last := state[len(state)-1]
	if uint(last) <= d.distance {
		return true
	}
	return false
}

func (d *dynamicLevenshtein) canMatch(state []int) bool {
	distance := int(d.distance)
	for _, v := range state {
		if v <= distance {
			return true
		}
	}
	return false
}

func (d *dynamicLevenshtein) accept(state []int, r *rune) []int {
	next := make([]int, 0, len(d.query)+1)
	next = append(next, state[0]+1)
	i := 0
	for _, c := range d.query {
		var cost int
		if r == nil || c != *r {
			cost = 1
		}
		v := min(min(next[i]+1, state[i+1]+1), state[i]+cost)
		next = append(next, min(v, int(d.distance)+1))
		i++
	}
	return next
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
