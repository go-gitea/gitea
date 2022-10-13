// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

import (
	"strings"
	"testing"
)

func TestBreakWriter_Write(t *testing.T) {
	tests := []struct {
		name    string
		kase    string
		expect  string
		wantErr bool
	}{
		{
			name:   "noline",
			kase:   "abcdefghijklmnopqrstuvwxyz",
			expect: "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:   "endline",
			kase:   "abcdefghijklmnopqrstuvwxyz\n",
			expect: "abcdefghijklmnopqrstuvwxyz<br>",
		},
		{
			name:   "startline",
			kase:   "\nabcdefghijklmnopqrstuvwxyz",
			expect: "<br>abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:   "onlyline",
			kase:   "\n\n\n",
			expect: "<br><br><br>",
		},
		{
			name:   "empty",
			kase:   "",
			expect: "",
		},
		{
			name:   "midline",
			kase:   "\nabc\ndefghijkl\nmnopqrstuvwxy\nz",
			expect: "<br>abc<br>defghijkl<br>mnopqrstuvwxy<br>z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &strings.Builder{}
			b := &BreakWriter{
				Writer: buf,
			}
			n, err := b.Write([]byte(tt.kase))
			if (err != nil) != tt.wantErr {
				t.Errorf("BreakWriter.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if n != len(tt.kase) {
				t.Errorf("BreakWriter.Write() = %v, want %v", n, len(tt.kase))
			}
			if buf.String() != tt.expect {
				t.Errorf("BreakWriter.Write() wrote %q, want %v", buf.String(), tt.expect)
			}
		})
	}
}
