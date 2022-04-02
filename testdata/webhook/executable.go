// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Payload struct {
	Form struct {
		Username       string `json:"username"`
		FavoriteNumber int    `json:"favorite-number"`
		Pineapple      bool   `json:"pineapple"`
		Passphrase     string `json:"passphrase"`
	} `json:"form"`
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		panic(err)
	}

	if p.Form.Username != "jolheiser" {
		panic("username should be jolheiser")
	}

	if p.Form.FavoriteNumber != 12 {
		panic("favNum should be 12")
	}

	if p.Form.Pineapple {
		panic("pineapple should be false")
	}

	if p.Form.Passphrase != "sn34ky" {
		panic("secret should be sn34ky")
	}
}

// override panic
func panic(v interface{}) {
	fmt.Println(v)
	os.Exit(1)
}
