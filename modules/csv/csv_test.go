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

//nolint
func TestCreateReaderAndGuessDelimiter(t *testing.T) {
	var csvToRowsMap = map[string][][]string{
		`a;b;c
1;2;3
4;5;6`: {{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5", "6"}},
		`col1	col2	col3
a	b	c
	e	f
g	h	i
j		l
m	n	
p	q	r
		u
v	w	x
y		
		`: {{"col1", "col2", "col3"},
			{"a", "b", "c"},
			{"", "e", "f"},
			{"g", "h", "i"},
			{"j", "", "l"},
			{"m", "n", ""},
			{"p", "q", "r"},
			{"", "", "u"},
			{"v", "w", "x"},
			{"y", "", ""},
			{"", "", ""}},
		` col1,col2,col3
 a, b, c
d,e,f
 ,h, i
j, , 
 , , `: {{"col1", "col2", "col3"},
			{"a", "b", "c"},
			{"d", "e", "f"},
			{"", "h", "i"},
			{"j", "", ""},
			{"", "", ""}},
	}

	for csv, expectedRows := range csvToRowsMap {
		rd, err := CreateReaderAndGuessDelimiter(strings.NewReader(csv))
		assert.NoError(t, err)
		rows, err := rd.ReadAll()
		assert.NoError(t, err)
		assert.EqualValues(t, rows, expectedRows)
	}
}

func TestGuessDelimiter(t *testing.T) {
	var csvToDelimiterMap = map[string]rune{
		"a":                         ',',
		"1,2":                       ',',
		"1;2":                       ';',
		"1\t2":                      '\t',
		"1|2":                       '|',
		"1,2,3;4,5,6;7,8,9\na;b;c":  ';',
		"\"1,2,3,4\";\"a\nb\"\nc;d": ';',
		"<br/>":                     ',',
	}

	for csv, expectedDelimiter := range csvToDelimiterMap {
		assert.EqualValues(t, guessDelimiter([]byte(csv)), expectedDelimiter)
	}
}
