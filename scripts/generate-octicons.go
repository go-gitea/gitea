// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gitea.com/jolheiser/octicon"

	"golang.org/x/net/html"
)

const outFile = "public/img/svg/octicons.svg"

func main() {
	if err := generateOcticons(); err != nil {
		fmt.Printf("could not generate Octicons: %v\n", err)
	}
}

func generateOcticons() error {
	sprite := `<svg xmlns="http://www.w3.org/2000/svg" style="display: none;">`
	for _, name := range octicon.Icons {
		icon := octicon.Icon(name)

		page, err := html.Parse(bytes.NewBufferString(icon.XML))
		if err != nil {
			return err
		}
		node := page.FirstChild.LastChild.FirstChild

		var viewBox html.Attribute
		for _, attr := range node.Attr {
			if attr.Key == "viewBox" {
				viewBox = attr
			}
		}
		node.Attr = []html.Attribute{viewBox}
		xml := &strings.Builder{}
		if err := html.Render(xml, node); err != nil {
			return err
		}

		sprite += fmt.Sprintf("\n"+`<symbol id="%s"%ssymbol>`, icon.Name, xml.String()[4:len(xml.String())-4])
	}
	sprite += "\n</svg>"

	if err := ioutil.WriteFile(outFile, []byte(sprite), os.ModePerm); err != nil {
		return err
	}
	return nil
}
