// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCsvReader(t *testing.T) {
	rd := CreateCsvReader([]byte{}, ',')
	assert.Equal(t, ',', rd.Comma)
}

func TestCreateCsvReaderAndGuessDelimiter(t *testing.T) {
	input := "a;b;c\n1;2;3\n4;5;6"

	rd := CreateCsvReaderAndGuessDelimiter([]byte(input))
	assert.Equal(t, ';', rd.Comma)
}

func TestGuessDelimiter(t *testint.T) {
	var kases = map[string]rune{
		"a":                         ',',
		"1,2":                       ',',
		"1;2":                       ';',
		"1\t2":                      '\t',
		"1|2":                       '|',
		"1,2,3;4,5,6;7,8,9\na;b;c":  ';',
		"\"1,2,3,4\";\"a\nb\"\nc;d": ';',
		"<br/>":                     ',',
	}

	for k, v := range kases {
		assert.EqualValues(t, guessDelimiter([]byte(k)), v)
	}
}