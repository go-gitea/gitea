// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package content

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"

	"code.gitea.io/gitea/modules/git/service"
)

// This file contains content functions for git Blobs

// GetBlobContent Gets the content of the blob as raw text
func GetBlobContent(b service.Object) (string, error) {
	dataRc, err := b.Reader()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]
	return string(buf), nil
}

// GetBlobLineCount gets line count of lob as raw text
func GetBlobLineCount(b service.Object) (int, error) {
	reader, err := b.Reader()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}
	for {
		c, err := reader.Read(buf)
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, err
		}
	}
}

// GetBlobContentBase64 Reads the content of the blob with a base64 encode and returns the encoded string
func GetBlobContentBase64(b service.Object) (string, error) {
	dataRc, err := b.Reader()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	pr, pw := io.Pipe()
	encoder := base64.NewEncoder(base64.StdEncoding, pw)

	go func() {
		_, err := io.Copy(encoder, dataRc)
		_ = encoder.Close()

		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
