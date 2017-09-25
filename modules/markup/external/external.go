// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package external

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	gouuid "github.com/satori/go.uuid"
)

// RegisterParsers registers all supported third part parsers according settings
func RegisterParsers() {
	for _, render := range setting.ExternalMarkupRenders {
		if render.Enabled && render.Command != "" && len(render.FileExtensions) > 0 {
			markup.RegisterParser(&Parser{render})
		}
	}
}

// Parser implements markup.Parser for external tools
type Parser struct {
	setting.MarkupRender
}

// Name returns the external tool name
func (p *Parser) Name() string {
	return p.MarkupName
}

// Extensions returns the supported extensions of the tool
func (p *Parser) Extensions() []string {
	return p.FileExtensions
}

// Render renders the data of the document to HTML via the external tool.
func (p *Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	var (
		bs       []byte
		buf      = bytes.NewBuffer(bs)
		rd       = bytes.NewReader(rawBytes)
		commands = strings.Fields(p.Command)
		args     = commands[1:]
	)

	if p.IsInputFile {
		// write to templ file
		fPath := filepath.Join(os.TempDir(), gouuid.NewV4().String())
		f, err := os.Create(fPath)
		if err != nil {
			log.Error(4, "%s create temp file when rendering %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}

		_, err = io.Copy(f, rd)
		f.Close()
		os.Remove(fPath)
		if err != nil {
			log.Error(4, "%s write data to temp file when rendering %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}
		args = append(args, fPath)
	}

	cmd := exec.Command(commands[0], args...)
	if !p.IsInputFile {
		cmd.Stdin = rd
	}
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		log.Error(4, "%s render run command %s failed: %v", p.Name(), p.Command, err)
		return []byte("")
	}
	return buf.Bytes()
}
