// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"testing"

	"code.gitea.io/gitea/models"
)

func Test_fixUnitConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		wantFixed bool
		wantErr   bool
	}{
		{
			name:      "empty",
			bs:        "",
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "normal: {}",
			bs:        "{}",
			wantFixed: false,
			wantErr:   false,
		},
		{
			name:      "broken but fixable: &{}",
			bs:        "&{}",
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "broken but unfixable: &{asdasd}",
			bs:        "&{asdasd}",
			wantFixed: false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFixed, err := fixUnitConfig_16961([]byte(tt.bs), &models.UnitConfig{})
			if (err != nil) != tt.wantErr {
				t.Errorf("fixUnitConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixUnitConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
		})
	}
}

func Test_fixExternalWikiConfig_16961(t *testing.T) {
	tests := []struct {
		name      string
		bs        string
		wantFixed bool
		wantErr   bool
	}{
		{
			name:      "normal: {\"ExternalWikiURL\":\"http://someurl\"}",
			bs:        "{\"ExternalWikiURL\":\"http://someurl\"}",
			wantFixed: false,
			wantErr:   false,
		},
		{
			name:      "broken: &{http://someurl}",
			bs:        "&{http://someurl}",
			wantFixed: true,
			wantErr:   false,
		},
		{
			name:      "broken but unfixable: http://someurl",
			bs:        "http://someurl",
			wantFixed: false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFixed, err := fixExternalWikiConfig_16961([]byte(tt.bs), &models.ExternalWikiConfig{})
			if (err != nil) != tt.wantErr {
				t.Errorf("fixExternalWikiConfig_16961() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFixed != tt.wantFixed {
				t.Errorf("fixExternalWikiConfig_16961() = %v, want %v", gotFixed, tt.wantFixed)
			}
		})
	}
}
