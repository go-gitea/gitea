// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package eventsource

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_wrapNewlines(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		value  string
		output string
	}{
		{
			"check no new lines",
			"prefix: ",
			"value",
			"prefix: value\n",
		},
		{
			"check simple newline",
			"prefix: ",
			"value1\nvalue2",
			"prefix: value1\nprefix: value2\n",
		},
		{
			"check pathological newlines",
			"p: ",
			"\n1\n\n2\n3\n",
			"p: \np: 1\np: \np: 2\np: 3\np: \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			gotSum, err := wrapNewlines(w, []byte(tt.prefix), []byte(tt.value))
			require.NoError(t, err)

			assert.EqualValues(t, len(tt.output), gotSum)
			assert.Equal(t, tt.output, w.String())
		})
	}
}
