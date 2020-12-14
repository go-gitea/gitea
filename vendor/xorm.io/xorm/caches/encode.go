// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caches

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

// md5 hash string
func Md5(str string) string {
	m := md5.New()
	io.WriteString(m, str)
	return fmt.Sprintf("%x", m.Sum(nil))
}
func Encode(data interface{}) ([]byte, error) {
	//return JsonEncode(data)
	return GobEncode(data)
}

func Decode(data []byte, to interface{}) error {
	//return JsonDecode(data, to)
	return GobDecode(data, to)
}

func GobEncode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GobDecode(data []byte, to interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(to)
}

func JsonEncode(data interface{}) ([]byte, error) {
	val, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func JsonDecode(data []byte, to interface{}) error {
	return json.Unmarshal(data, to)
}
