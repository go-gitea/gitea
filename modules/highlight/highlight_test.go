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
		want     map[int]string
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
			want: map[int]string{
				1: `<span class="nt">kind</span><span class="p">:</span><span class="w"> </span><span class="l">pipeline</span>`,
				2: `<span class="w"></span><span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">default</span>`,
				3: `<span class="w">
</span>`,
				4: `<span class="w"></span><span class="nt">steps</span><span class="p">:</span>`,
				5: `<span class="w"></span>- <span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">test</span>`,
				6: `<span class="w">	</span><span class="nt">image</span><span class="p">:</span><span class="w"> </span><span class="l">golang:1.13</span>`,
				7: `<span class="w">	</span><span class="nt">environment</span><span class="p">:</span>`,
				8: `<span class="w"></span><span class="w">		</span><span class="nt">GOPROXY</span><span class="p">:</span><span class="w"> </span><span class="l">https://goproxy.cn</span>`,
				9: `<span class="w">	</span><span class="nt">commands</span><span class="p">:</span>`,
				10: `<span class="w"></span><span class="w">	</span>- <span class="l">go get -u</span>`,
				11: `<span class="w">	</span>- <span class="l">go build -v</span>`,
				12: `<span class="w">	</span>- <span class="l">go test -v -race -coverprofile=coverage.txt -covermode=atomic</span><span class="w">
</span>`,
				13: `<span class="w">
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
			want: map[int]string{
				1: `<span class="nt">kind</span><span class="p">:</span><span class="w"> </span><span class="l">pipeline</span>`,
				2: `<span class="w"></span><span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">default  </span>`,
				3: `<span class="w">
</span>`,
				4: `<span class="w"></span><span class="nt">steps</span><span class="p">:</span>`,
				5: `<span class="w"></span>- <span class="nt">name</span><span class="p">:</span><span class="w"> </span><span class="l">test</span>`,
				6: `<span class="w">	</span><span class="nt">image</span><span class="p">:</span><span class="w"> </span><span class="l">golang:1.13</span>`,
				7: `<span class="w">	</span><span class="nt">environment</span><span class="p">:</span>`,
				8: `<span class="w"></span><span class="w">		</span><span class="nt">GOPROXY</span><span class="p">:</span><span class="w"> </span><span class="l">https://goproxy.cn</span>`,
				9: `<span class="w">	</span><span class="nt">commands</span><span class="p">:</span>`,
				10: `<span class="w"></span><span class="w">	</span>- <span class="l">go get -u</span>`,
				11: `<span class="w">	</span>- <span class="l">go build -v</span>`,
				12: `<span class="w">	</span>- <span class="l">go test -v -race -coverprofile=coverage.txt -covermode=atomic</span>`,
				13: `<span class="w">	</span>`,
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
