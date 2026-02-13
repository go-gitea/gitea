// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ForgeDirs(t *testing.T) {
	oldRepository := Repository
	defer func() {
		Repository = oldRepository
	}()

	tests := []struct {
		name     string
		iniStr   string
		wantDirs []string
	}{
		{
			name:     "default",
			iniStr:   `[repository]`,
			wantDirs: []string{".gitea", ".github"},
		},
		{
			name:     "single dir",
			iniStr:   "[repository]\nFORGE_DIRS = .github",
			wantDirs: []string{".github"},
		},
		{
			name:     "custom order",
			iniStr:   "[repository]\nFORGE_DIRS = .github,.gitea",
			wantDirs: []string{".github", ".gitea"},
		},
		{
			name:     "whitespace trimming",
			iniStr:   "[repository]\nFORGE_DIRS = .gitea , .github ",
			wantDirs: []string{".gitea", ".github"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			require.NoError(t, err)
			loadRepositoryFrom(cfg)
			assert.Equal(t, tt.wantDirs, Repository.ForgeDirs)
		})
	}
}
