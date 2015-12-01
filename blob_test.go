// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io/ioutil"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

var testBlob = &Blob{
	repo: &Repository{},
	TreeEntry: &TreeEntry{
		ID: MustIDFromString("176d8dfe018c850d01851b05fb8a430096247353"),
	},
}

func Test_Blob_Data(t *testing.T) {
	Convey("Get blob data", t, func() {
		_output := `Copyright (c) 2015 All Gogs Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.`

		Convey("Get data all at once", func() {
			r, err := testBlob.Data()
			So(err, ShouldBeNil)
			So(r, ShouldNotBeNil)

			data, err := ioutil.ReadAll(r)
			So(err, ShouldBeNil)
			So(string(data), ShouldEqual, _output)
		})

		Convey("Get blob data with pipeline", func() {
			stdout := new(bytes.Buffer)
			err := testBlob.DataPipeline(stdout, nil)
			So(err, ShouldBeNil)
			So(stdout.String(), ShouldEqual, _output)
		})
	})
}

func Benchmark_Blob_Data(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, _ := testBlob.Data()
		ioutil.ReadAll(r)
	}
}

func Benchmark_Blob_DataPipeline(b *testing.B) {
	stdout := new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		testBlob.DataPipeline(stdout, nil)
	}
}
