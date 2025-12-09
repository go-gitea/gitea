// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getLicense(t *testing.T) {
	type args struct {
		name   string
		values *LicenseValues
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "regular",
			args: args{
				name:   "MIT",
				values: &LicenseValues{Owner: "Gitea", Year: "2023"},
			},
			want: `MIT License

Copyright (c) 2023 Gitea

Permission is hereby granted`,
			wantErr: assert.NoError,
		},
		{
			name: "license not found",
			args: args{
				name: "notfound",
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLicense(tt.args.name, tt.args.values)
			if !tt.wantErr(t, err, fmt.Sprintf("GetLicense(%v, %v)", tt.args.name, tt.args.values)) {
				return
			}
			assert.Contains(t, string(got), tt.want, "GetLicense(%v, %v)", tt.args.name, tt.args.values)
		})
	}
}

func Test_fillLicensePlaceholder(t *testing.T) {
	type args struct {
		name   string
		values *LicenseValues
		origin string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "owner",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
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
			},
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
			name: "email",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
[EMAIL]
`,
			},
			want: `
teabot@gitea.io
`,
		},
		{
			name: "repo",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
<program>
<one line to give the program's name and a brief idea of what it does.>
`,
			},
			want: `
gitea
gitea
`,
		},
		{
			name: "year",
			args: args{
				name:   "regular",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
<year>
[YEAR]
{YEAR}
[yyyy]
[Year]
[year]
`,
			},
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
			name: "0BSD",
			args: args{
				name:   "0BSD",
				values: &LicenseValues{Year: "2023", Owner: "Gitea", Email: "teabot@gitea.io", Repo: "gitea"},
				origin: `
Copyright (C) YEAR by AUTHOR EMAIL

...

... THE AUTHOR BE LIABLE FOR ...
`,
			},
			want: `
Copyright (C) 2023 by Gitea teabot@gitea.io

...

... THE AUTHOR BE LIABLE FOR ...
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, string(fillLicensePlaceholder(tt.args.name, tt.args.values, []byte(tt.args.origin))), "fillLicensePlaceholder(%v, %v, %v)", tt.args.name, tt.args.values, tt.args.origin)
		})
	}
}
