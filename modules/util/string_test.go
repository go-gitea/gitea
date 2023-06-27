// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase(t *testing.T) {
	cases := map[string]string{
		// all old cases from the legacy package
		"HTTPServer":         "http_server",
		"_camelCase":         "_camel_case",
		"NoHTTPS":            "no_https",
		"Wi_thF":             "wi_th_f",
		"_AnotherTES_TCaseP": "_another_tes_t_case_p",
		"ALL":                "all",
		"_HELLO_WORLD_":      "_hello_world_",
		"HELLO_WORLD":        "hello_world",
		"HELLO____WORLD":     "hello____world",
		"TW":                 "tw",
		"_C":                 "_c",

		"  sentence case  ": "__sentence_case__",
		" Mixed-hyphen case _and SENTENCE_case and UPPER-case": "_mixed_hyphen_case__and_sentence_case_and_upper_case",

		// new cases
		" ":        "_",
		"A":        "a",
		"A0":       "a0",
		"a0":       "a0",
		"Aa0":      "aa0",
		"啊":        "啊",
		"A啊":       "a啊",
		"Aa啊b":     "aa啊b",
		"A啊B":      "a啊_b",
		"Aa啊B":     "aa啊_b",
		"TheCase2": "the_case2",
		"ObjIDs":   "obj_i_ds", // the strange database column name which already exists
	}
	for input, expected := range cases {
		assert.Equal(t, expected, ToSnakeCase(input))
	}
}

type (
	testSliceUnionInput[T string | int]  [][]T
	testSliceUnionOutput[T string | int] []T
	testUnionItem[T string | int]        struct {
		input    testSliceUnionInput[T]
		expected testSliceUnionOutput[T]
	}
)

func TestSliceUnion(t *testing.T) {
	intTests := []testUnionItem[int]{
		{
			input: testSliceUnionInput[int]{
				[]int{1, 2, 2, 3},
				[]int{2, 4, 7},
			},
			expected: []int{1, 2, 3, 4, 7},
		},
		{
			input: testSliceUnionInput[int]{
				[]int{7, 8, 1},
				[]int{1, 2, 3},
				[]int{3, 4, 5},
			},
			expected: []int{1, 2, 3, 4, 5, 7, 8},
		},
	}
	for i, test := range intTests {
		t.Run(fmt.Sprintf("int test: %d", i), func(t *testing.T) {
			actual := SliceUnion(test.input...)
			// sort
			sort.Ints(actual)
			assert.EqualValues(t, test.expected, actual, actual)
		})
	}

	stringTests := []testUnionItem[string]{
		{
			input: testSliceUnionInput[string]{
				[]string{"a", "c"},
				[]string{"c", "d", "a", "b"},
			},
			expected: []string{"a", "b", "c", "d"},
		},
	}
	for i, test := range stringTests {
		t.Run(fmt.Sprintf("string test: %d", i), func(t *testing.T) {
			actual := SliceUnion(test.input...)
			// sort
			sort.Strings(actual)
			assert.EqualValues(t, test.expected, actual, actual)
		})
	}
}
