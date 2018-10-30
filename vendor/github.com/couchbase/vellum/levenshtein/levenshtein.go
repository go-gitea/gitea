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
	"fmt"
)

// StateLimit is the maximum number of states allowed
const StateLimit = 10000

// ErrTooManyStates is returned if you attempt to build a Levenshtein
// automaton which requires too many states.
var ErrTooManyStates = fmt.Errorf("dfa contains more than %d states", StateLimit)

// Levenshtein implements the vellum.Automaton interface for matching
// terms within the specified Levenshtein edit-distance of the queried
// term.  This automaton recognizes utf-8 encoded bytes and computes
// the edit distance on the result code-points, not on the raw bytes.
type Levenshtein struct {
	prog *dynamicLevenshtein
	dfa  *dfa
}

// New creates a new Levenshtein automaton for the specified
// query string and edit distance.
func New(query string, distance int) (*Levenshtein, error) {
	lev := &dynamicLevenshtein{
		query:    query,
		distance: uint(distance),
	}
	dfabuilder := newDfaBuilder(lev)
	dfa, err := dfabuilder.build()
	if err != nil {
		return nil, err
	}
	return &Levenshtein{
		prog: lev,
		dfa:  dfa,
	}, nil
}

// Start returns the start state of this automaton.
func (l *Levenshtein) Start() int {
	return 1
}

// IsMatch returns if the specified state is a matching state.
func (l *Levenshtein) IsMatch(s int) bool {
	if s < len(l.dfa.states) {
		return l.dfa.states[s].match
	}
	return false
}

// CanMatch returns if the specified state can ever transition to a matching
// state.
func (l *Levenshtein) CanMatch(s int) bool {
	if s < len(l.dfa.states) && s > 0 {
		return true
	}
	return false
}

// WillAlwaysMatch returns if the specified state will always end in a
// matching state.
func (l *Levenshtein) WillAlwaysMatch(s int) bool {
	return false
}

// Accept returns the new state, resulting from the transite byte b
// when currently in the state s.
func (l *Levenshtein) Accept(s int, b byte) int {
	if s < len(l.dfa.states) {
		return l.dfa.states[s].next[b]
	}
	return 0
}
