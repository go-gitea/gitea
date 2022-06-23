// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package mph

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkConstruction(b *testing.B) {
	// Make around 5k keys.
	keys := make([]string, 5_000)
	buffer := make([]byte, 4)
	for i := uint32(0); i < 5_000; i++ {
		binary.LittleEndian.PutUint32(buffer, i)
		keys[i] = string(buffer)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Build(keys)
	}
}

func TestPerfectHashFunction_Random(t *testing.T) {
	// Make around 5k keys.
	keys := make([]string, 5_000)
	buffer := make([]byte, 4)
	for i := uint32(0); i < 5_000; i++ {
		binary.LittleEndian.PutUint32(buffer, i)
		keys[i] = string(buffer)
	}

	// Build the perfect hash.
	phf := Build(keys)

	buffer = make([]byte, 4)

	// Check if the given indexes correspond to the correct key.
	for i := uint32(0); i < 5_000; i++ {
		binary.LittleEndian.PutUint32(buffer, i)
		assert.Equal(t, i, phf.Get(string(buffer)))
	}
}

var uniqueStrings = []string{
	"Hello, world", "FOSS", "Gitea", "gi21t", "gamining", "emacs", "slices",
	"pointers", "bit twidling", "😀", "e ee ", "lolz", "A quite long sentence!",
}

func TestPerfectHashFunction_Static(t *testing.T) {
	// Build the perfect hash.
	phf := Build(uniqueStrings)

	// Check if the given indexes correspond to the correct key.
	for i := 0; i < len(uniqueStrings); i++ {
		assert.Equal(t, uint32(i), phf.Get(uniqueStrings[i]))
	}
}
