// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package blenderid

import "testing"

func Test_gitealizeUsername(t *testing.T) {
	tests := []struct {
		name        string
		bidNickname string
		want        string
	}{
		{"empty", "", ""},
		{"underscore", "_", "_"},
		{"reserved-name", "ghost", "ghost2"}, // Reserved name in Gitea.
		{"short", "x", "x"},
		{"simple", "simple", "simple"},
		{"start-bad", "____startbad", "startbad"},
		{"end-bad", "endbad___", "endbad"},
		{"mid-bad-1", "mid__bad", "mid_bad"},
		{"mid-bad-2", "user_.-name", "user_name"},
		{"plus-mid-single", "RT2+356", "RT2_356"},
		{"plus-mid-many", "RT2+++356", "RT2_356"},
		{"plus-end", "RT2356+", "RT2356"},
		{
			"too-long", // # Max username length is 40:
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{"accented-latin", "Ümlaut-Đenja", "Umlaut-Denja"},
		{"thai", "แบบไทย", "aebbaithy"},
		{"mandarin", "普通话", "Pu_Tong_Hua"},
		{"cyrillic", "ћирилица", "tshirilitsa"},
		{"all-bad", "------", "------"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitealizeUsername(tt.bidNickname); got != tt.want {
				t.Errorf("gitealizeUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}
