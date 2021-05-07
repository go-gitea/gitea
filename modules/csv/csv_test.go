// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package csv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateReader(t *testing.T) {
	rd := CreateReader(bytes.NewReader([]byte{}), ',')
	assert.Equal(t, ',', rd.Comma)
}

func TestCreateReaderAndGuessDelimiter(t *testing.T) {
	input := "a;b;c\n1;2;3\n4;5;6"

	rd, err := CreateReaderAndGuessDelimiter(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Equal(t, ';', rd.Comma)
}

func TestGuessDelimiter(t *testing.T) {
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
