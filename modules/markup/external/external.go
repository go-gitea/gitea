// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package external

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

// RegisterParsers registers all supported third part parsers according settings
func RegisterParsers() {
	for _, parser := range setting.ExternalMarkupParsers {
		if parser.Enabled && parser.Command != "" && len(parser.FileExtensions) > 0 {
			markup.RegisterParser(&Parser{parser})
		}
	}
}

// Parser implements markup.Parser for external tools
type Parser struct {
	setting.MarkupParser
}

// Name returns the external tool name
func (p *Parser) Name() string {
	return p.MarkupName
}

// Extensions returns the supported extensions of the tool
func (p *Parser) Extensions() []string {
	return p.FileExtensions
}

func envMark(envName string) string {
	if runtime.GOOS == "windows" {
		return "%" + envName + "%"
	}
	return "$" + envName
}

// Render renders the data of the document to HTML via the external tool.
func (p *Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	var (
		bs           []byte
		buf          = bytes.NewBuffer(bs)
		rd           = bytes.NewReader(rawBytes)
		urlRawPrefix = strings.Replace(urlPrefix, "/src/", "/raw/", 1)

		command = strings.NewReplacer(envMark("GITEA_PREFIX_SRC"), urlPrefix,
			envMark("GITEA_PREFIX_RAW"), urlRawPrefix).Replace(p.Command)
		commands = strings.Fields(command)
		args     = commands[1:]
	)

	if p.IsInputFile {
		// write to temp file
		f, err := ioutil.TempFile("", "gitea_input")
		if err != nil {
			log.Error("%s create temp file when rendering %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}
		defer os.Remove(f.Name())

		_, err = io.Copy(f, rd)
		if err != nil {
			f.Close()
			log.Error("%s write data to temp file when rendering %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}

		err = f.Close()
		if err != nil {
			log.Error("%s close temp file when rendering %s failed: %v", p.Name(), p.Command, err)
			return []byte("")
		}
		args = append(args, f.Name())
	}

	cmd := exec.Command(commands[0], args...)
	cmd.Env = append(
		os.Environ(),
		"GITEA_PREFIX_SRC="+urlPrefix,
		"GITEA_PREFIX_RAW="+urlRawPrefix,
	)
	if !p.IsInputFile {
		cmd.Stdin = rd
	}
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		log.Error("%s render run command %s %v failed: %v", p.Name(), commands[0], args, err)
		return []byte("")
	}
	return buf.Bytes()
}
