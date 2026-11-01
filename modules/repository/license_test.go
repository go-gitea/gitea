// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getLicense(t *testing.T) {
	tests := []struct {
		name    string
		license string
		values  *LicenseValues
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "regular",
			license: "MIT",
			values:  &LicenseValues{Owner: "Gitea", Year: "2023"},
			want: `MIT License

Copyright (c) 2023 Gitea

Permission is hereby granted`,
			wantErr: assert.NoError,
		},
		{
			name:    "license not found",
			license: "notfound",
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLicense(tt.license, tt.values)
			if !tt.wantErr(t, err, fmt.Sprintf("GetLicense(%v, %v)", tt.license, tt.values)) {
				return
			}
			assert.Contains(t, string(got), tt.want, "GetLicense(%v, %v)", tt.license, tt.values)
		})
	}
}

func Test_fillLicensePlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		license string
		origin  string
		want    string
	}{
		{
			name:    "owner",
			license: "regular",
			origin: `
<name of author>
<owner>
[NAME]
[name of copyright owner]
[name of copyright holder]
<COPYRIGHT HOLDERS>
<copyright holders>
<AUTHOR>
<author's name or designee>
[one or more legally recognised persons or entities offering the Work under the terms and conditions of this Licence]
`,
			want: `
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
Gitea
`,
		},
		{
			name:    "email",
			license: "regular",
			origin: `
[EMAIL]
`,
			want: `
teabot@gitea.io
`,
		},
		{
			name:    "repo",
			license: "regular",
			origin: `
<program>
<one line to give the program's name and a brief idea of what it does.>
`,
			want: `
gitea
gitea
`,
		},
		{
			name:    "year",
			license: "regular",
			origin: `
<year>
[YEAR]
{YEAR}
[yyyy]
[Year]
[year]
`,
			want: `
2023
2023
2023
2023
2023
2023
`,
		},
		{
			name:    "0BSD",
			license: "0BSD",
			origin: `
Copyright (C) YEAR by AUTHOR EMAIL

...

... THE AUTHOR BE LIABLE FOR ...
`,
			want: `
Copyright (C) 2023 by Gitea teabot@gitea.io

...

... THE AUTHOR BE LIABLE FOR ...
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"}
			assert.Equalf(t, tt.want, string(fillLicensePlaceholder(tt.license, values, []byte(tt.origin))), "fillLicensePlaceholder(%v, %v, %v)", tt.license, values, tt.origin)
		})
	}
}
