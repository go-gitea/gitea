// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
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

func TestSplitTrimSpace(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, SplitTrimSpace("a\nb\nc", "\n"))
	assert.Equal(t, []string{"a", "b"}, SplitTrimSpace("\r\na\n\r\nb\n\n", "\n"))
}
