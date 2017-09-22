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

// Parser implements markup.Parser for orgmode
type Parser struct {
	setting.MarkupRender
}

// Name implements markup.Parser
func (p *Parser) Name() string {
	return p.MarkupName
}

// Extensions implements markup.Parser
func (p *Parser) Extensions() []string {
	return p.FileExtensions
}

// Render implements markup.Parser
func (p *Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	var bs []byte
	var buf = bytes.NewBuffer(bs)
	var rd = bytes.NewReader(rawBytes)
	commands := strings.Fields(p.Command)
	var command = commands[0]
	var args []string
	if len(commands) > 1 {
		args = commands[1:]
	}
	if p.IsInputFile {
		// write to templ file
		fPath := filepath.Join(os.TempDir(), gouuid.NewV4().String())
		f, err := os.Create(fPath)
		if err != nil {
			log.Error(4, "%s render run command %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}

		_, err = io.Copy(f, rd)
		f.Close()
		if err != nil {
			log.Error(4, "%s render run command %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}
		args = append(args, fPath)
	}

	cmd := exec.Command(command, args...)
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
