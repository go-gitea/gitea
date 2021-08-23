// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package highlight

import (
	"reflect"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"gopkg.in/ini.v1"
)

func TestFile(t *testing.T) {
	setting.Cfg = ini.Empty()
	tests := []struct {
		name     string
		numLines int
		fileName string
		code     string
		want     []string
	}{
		{
			name:     ".drone.yml",
			numLines: 12,
			fileName: ".drone.yml",
			code: `kind: pipeline
name: default

steps:
- name: test
	image: golang:1.13
	environment:
		GOPROXY: https://goproxy.cn
	commands:
	- go get -u
	- go build -v
	- go test -v -race -coverprofile=coverage.txt -covermode=atomic
`,
			want: []string{
				`<span class="nt">kind</span><span class="p">:</span><span class="w"> </span><span class="l">pipeline</span>`,
				`<span class="w"></span><span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">default</span>`,
				`<span class="w">
</span>`,
				`<span class="w"></span><span class="nt">steps</span><span class="p">:</span>`,
				`<span class="w"></span>- <span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">test</span>`,
				`<span class="w">	</span><span class="nt">image</span><span class="p">:</span><span class="w"> </span><span class="l">golang:1.13</span>`,
				`<span class="w">	</span><span class="nt">environment</span><span class="p">:</span>`,
				`<span class="w"></span><span class="w">		</span><span class="nt">GOPROXY</span><span class="p">:</span><span class="w"> </span><span class="l">https://goproxy.cn</span>`,
				`<span class="w">	</span><span class="nt">commands</span><span class="p">:</span>`,
				`<span class="w"></span><span class="w">	</span>- <span class="l">go get -u</span>`,
				`<span class="w">	</span>- <span class="l">go build -v</span>`,
				`<span class="w">	</span>- <span class="l">go test -v -race -coverprofile=coverage.txt -covermode=atomic</span><span class="w">
</span>`,
				`<span class="w">
</span>`,
			},
		},
		{
			name:     ".drone.yml - trailing space",
			numLines: 13,
			fileName: ".drone.yml",
			code: `kind: pipeline
name: default  ` + `

steps:
- name: test
	image: golang:1.13
	environment:
		GOPROXY: https://goproxy.cn
	commands:
	- go get -u
	- go build -v
	- go test -v -race -coverprofile=coverage.txt -covermode=atomic
	`,
			want: []string{
				`<span class="nt">kind</span><span class="p">:</span><span class="w"> </span><span class="l">pipeline</span>`,
				`<span class="w"></span><span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">default  </span>`,
				`<span class="w">
</span>`,
				`<span class="w"></span><span class="nt">steps</span><span class="p">:</span>`,
				`<span class="w"></span>- <span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">test</span>`,
				`<span class="w">	</span><span class="nt">image</span><span class="p">:</span><span class="w"> </span><span class="l">golang:1.13</span>`,
				`<span class="w">	</span><span class="nt">environment</span><span class="p">:</span>`,
				`<span class="w"></span><span class="w">		</span><span class="nt">GOPROXY</span><span class="p">:</span><span class="w"> </span><span class="l">https://goproxy.cn</span>`,
				`<span class="w">	</span><span class="nt">commands</span><span class="p">:</span>`,
				`<span class="w"></span><span class="w">	</span>- <span class="l">go get -u</span>`,
				`<span class="w">	</span>- <span class="l">go build -v</span>`,
				`<span class="w">	</span>- <span class="l">go test -v -race -coverprofile=coverage.txt -covermode=atomic</span>`,
				`<span class="w">	</span>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := File(tt.numLines, tt.fileName, []byte(tt.code)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("File() = %v, want %v", got, tt.want)
			}
		})
	}
}
