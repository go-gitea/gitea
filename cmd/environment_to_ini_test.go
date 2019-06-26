// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import "testing"

func TestDecodeSectionKey(t *testing.T) {
	type args struct {
		encoded string
	}
	tests := []struct {
		name    string
		encoded string
		section string
		key     string
	}{
		{
			name:    "Simple",
			encoded: "Section__Key",
			section: "Section",
			key:     "Key",
		},
		{
			name:    "LessSimple",
			encoded: "Section_SubSection__Key_SubKey",
			section: "Section_SubSection",
			key:     "Key_SubKey",
		},
		{
			name:    "OneDotOneDash",
			encoded: "Section_0X2E_SubSection__Key_0X2D_SubKey",
			section: "Section.SubSection",
			key:     "Key-SubKey",
		},
		{
			name:    "OneDotOneEncodedOneDash",
			encoded: "Section_0X2E_0X2E_Sub_0X2D_Section__Key_0X2D_SubKey",
			section: "Section.0X2E_Sub-Section",
			key:     "Key-SubKey",
		},
		{
			name:    "EncodedUnderscore",
			encoded: "Section__0X5F_0X2E_Sub_0X2D_Section__Key_0X2D__0X2D_SubKey",
			section: "Section__0X2E_Sub-Section",
			key:     "Key--SubKey",
		},
		{
			name:    "EncodedUtf8",
			encoded: "Section__0XE280A6_Sub_0X2D_Section__Key_0X2D__0X2D_SubKey",
			section: "Section_…Sub-Section",
			key:     "Key--SubKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSection, gotKey := DecodeSectionKey(tt.encoded)
			if gotSection != tt.section {
				t.Errorf("DecodeSectionKey() gotSection = %v, want %v", gotSection, tt.section)
			}
			if gotKey != tt.key {
				t.Errorf("DecodeSectionKey() gotKey = %v, want %v", gotKey, tt.key)
			}
		})
	}
}
