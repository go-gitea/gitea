// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// ToUTF8WithErr converts content to UTF8 encoding
func ToUTF8WithErr(content []byte) (string, error) {
	charsetLabel, err := DetectEncoding(content)
	if err != nil {
		return "", err
	} else if charsetLabel == "UTF-8" {
		return string(RemoveBOMIfPresent(content)), nil
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return string(content), fmt.Errorf("Unknown encoding: %s", charsetLabel)
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		result = append(result, content[n:]...)
	}

	result = RemoveBOMIfPresent(result)

	return string(result), err
}

// ToUTF8WithFallback detects the encoding of content and coverts to UTF-8 if possible
func ToUTF8WithFallback(content []byte) []byte {
	charsetLabel, err := DetectEncoding(content)
	if err != nil || charsetLabel == "UTF-8" {
		return RemoveBOMIfPresent(content)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return content
	}

	// If there is an error, we concatenate the nicely decoded part and the
	// original left over. This way we won't lose data.
	result, n, err := transform.Bytes(encoding.NewDecoder(), content)
	if err != nil {
		return append(result, content[n:]...)
	}

	return RemoveBOMIfPresent(result)
}

// ToUTF8 converts content to UTF8 encoding and ignore error
func ToUTF8(content string) string {
	res, _ := ToUTF8WithErr([]byte(content))
	return res
}

// ToUTF8DropErrors makes sure the return string is valid utf-8; attempts conversion if possible
func ToUTF8DropErrors(content []byte) []byte {
	charsetLabel, err := DetectEncoding(content)
	if err != nil || charsetLabel == "UTF-8" {
		return RemoveBOMIfPresent(content)
	}

	encoding, _ := charset.Lookup(charsetLabel)
	if encoding == nil {
		return content
	}

	// We ignore any non-decodable parts from the file.
	// Some parts might be lost
	var decoded []byte
	decoder := encoding.NewDecoder()
	idx := 0
	for {
		result, n, err := transform.Bytes(decoder, content[idx:])
		decoded = append(decoded, result...)
		if err == nil {
			break
		}
		decoded = append(decoded, ' ')
		idx = idx + n + 1
		if idx >= len(content) {
			break
		}
	}

	return RemoveBOMIfPresent(decoded)
}
