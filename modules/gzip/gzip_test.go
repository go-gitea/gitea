// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gzip

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.com/macaron/macaron"
	gzipp "github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
)

func setup(sampleResponse []byte) (*macaron.Macaron, *[]byte) {
	m := macaron.New()
	m.Use(Middleware())
	m.Get("/", func() *[]byte { return &sampleResponse })
	return m, &sampleResponse
}

func reqNoAcceptGzip(t *testing.T, m *macaron.Macaron, sampleResponse *[]byte) {
	// Request without accept gzip: Should not gzip
	resp := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)
	m.ServeHTTP(resp, req)

	_, ok := resp.HeaderMap[contentEncodingHeader]
	assert.False(t, ok)

	contentEncoding := resp.Header().Get(contentEncodingHeader)
	assert.NotContains(t, contentEncoding, "gzip")

	result := resp.Body.Bytes()
	assert.Equal(t, *sampleResponse, result)
}

func reqAcceptGzip(t *testing.T, m *macaron.Macaron, sampleResponse *[]byte, expectGzip bool) {
	// Request without accept gzip: Should not gzip
	resp := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)
	req.Header.Set(acceptEncodingHeader, "gzip")
	m.ServeHTTP(resp, req)

	_, ok := resp.HeaderMap[contentEncodingHeader]
	assert.Equal(t, ok, expectGzip)

	contentEncoding := resp.Header().Get(contentEncodingHeader)
	if expectGzip {
		assert.Contains(t, contentEncoding, "gzip")
		gzippReader, err := gzipp.NewReader(resp.Body)
		assert.NoError(t, err)
		result, err := ioutil.ReadAll(gzippReader)
		assert.NoError(t, err)
		assert.Equal(t, *sampleResponse, result)
	} else {
		assert.NotContains(t, contentEncoding, "gzip")
		result := resp.Body.Bytes()
		assert.Equal(t, *sampleResponse, result)
	}
}

func TestMiddlewareSmall(t *testing.T) {
	m, sampleResponse := setup([]byte("Small response"))

	reqNoAcceptGzip(t, m, sampleResponse)

	reqAcceptGzip(t, m, sampleResponse, false)
}

func TestMiddlewareLarge(t *testing.T) {
	b := make([]byte, MinSize+1)
	for i := range b {
		b[i] = byte(i % 256)
	}
	m, sampleResponse := setup(b)

	reqNoAcceptGzip(t, m, sampleResponse)

	// This should be gzipped as we accept gzip
	reqAcceptGzip(t, m, sampleResponse, true)
}

func TestMiddlewareGzip(t *testing.T) {
	b := make([]byte, MinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	gzippWriter := gzipp.NewWriter(outputBuffer)
	gzippWriter.Write(b)
	gzippWriter.Flush()
	gzippWriter.Close()
	output := outputBuffer.Bytes()

	m, sampleResponse := setup(output)

	reqNoAcceptGzip(t, m, sampleResponse)

	// This should not be gzipped even though we accept gzip
	reqAcceptGzip(t, m, sampleResponse, false)
}

func TestMiddlewareZip(t *testing.T) {
	b := make([]byte, MinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	zipWriter := zip.NewWriter(outputBuffer)
	fileWriter, err := zipWriter.Create("default")
	assert.NoError(t, err)
	fileWriter.Write(b)
	//fileWriter.Close()
	zipWriter.Close()
	output := outputBuffer.Bytes()

	m, sampleResponse := setup(output)

	reqNoAcceptGzip(t, m, sampleResponse)

	// This should not be gzipped even though we accept gzip
	reqAcceptGzip(t, m, sampleResponse, false)
}
