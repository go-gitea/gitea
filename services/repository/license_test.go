// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"strings"
	"testing"

	repo_module "code.gitea.io/gitea/modules/repository"

	"github.com/stretchr/testify/assert"
)

func Test_detectLicense(t *testing.T) {
	type DetectLicenseTest struct {
		name string
		arg  string
		want []string
	}

	tests := []DetectLicenseTest{
		{
			name: "empty",
			arg:  "",
			want: nil,
		},
		{
			name: "no detected license",
			arg:  "Copyright (c) 2023 Gitea",
			want: nil,
		},
	}

	repo_module.LoadRepoConfig()
	err := loadLicenseAliases()
	assert.NoError(t, err)
	for _, licenseName := range repo_module.Licenses {
		license, err := repo_module.GetLicense(licenseName, &repo_module.LicenseValues{
			Owner: "Gitea",
			Email: "teabot@gitea.io",
			Repo:  "gitea",
			Year:  "2024",
		})
		assert.NoError(t, err)

		tests = append(tests, DetectLicenseTest{
			name: fmt.Sprintf("single license test: %s", licenseName),
			arg:  string(license),
			want: []string{ConvertLicenseName(licenseName)},
		})
	}

	err = InitLicenseClassifier()
	assert.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			license, err := detectLicense(strings.NewReader(tt.arg))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, license)
		})
	}

	result, err := detectLicense(strings.NewReader(tests[2].arg + tests[3].arg + tests[4].arg))
	assert.NoError(t, err)
	t.Run("multiple licenses test", func(t *testing.T) {
		assert.Len(t, result, 3)
		assert.Contains(t, result, tests[2].want[0])
		assert.Contains(t, result, tests[3].want[0])
		assert.Contains(t, result, tests[4].want[0])
	})
}
