// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package content

import (
	"image"
	"image/color"
	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/git/service"
)

// ImageMetaData represents metadata of an image file
type ImageMetaData struct {
	ColorModel color.Model
	Width      int
	Height     int
	ByteSize   int64
}

func isImageFile(data []byte) (string, bool) {
	contentType := http.DetectContentType(data)
	if strings.Contains(contentType, "image/") {
		return contentType, true
	}
	return contentType, false
}

// IsImageFile is a file image type
func IsImageFile(c service.Commit, name string) bool {
	blob, err := c.Tree().GetBlobByPath(name)
	if err != nil {
		return false
	}

	dataRc, err := blob.Reader()
	if err != nil {
		return false
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]
	_, isImage := isImageFile(buf)
	return isImage
}

// ImageInfo returns information about the dimensions of an image
func ImageInfo(c service.Commit, name string) (*ImageMetaData, error) {
	if !IsImageFile(c, name) {
		return nil, nil
	}

	blob, err := c.Tree().GetBlobByPath(name)
	if err != nil {
		return nil, err
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	config, _, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}

	metadata := ImageMetaData{
		ColorModel: config.ColorModel,
		Width:      config.Width,
		Height:     config.Height,
		ByteSize:   blob.Size(),
	}
	return &metadata, nil
}
