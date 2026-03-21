// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"bytes"
	"html/template"
	"io"
)

type finalProcessor struct {
	renderInternal *RenderInternal
	extraHeadHTML  template.HTML

	output io.Writer
	buf    bytes.Buffer
}

func (p *finalProcessor) Write(data []byte) (int, error) {
	p.buf.Write(data)
	return len(data), nil
}

func (p *finalProcessor) Close() error {
	// TODO: reading the whole markdown isn't a problem at the moment,
	// because "postProcess" already does so. In the future we could optimize the code to process data on the fly.
	buf := p.buf.Bytes()
	buf = bytes.ReplaceAll(buf, []byte(` data-attr-class="`+p.renderInternal.secureIDPrefix), []byte(` class="`))

	tmp := bytes.TrimSpace(buf)
	isLikelyHTML := len(tmp) != 0 && tmp[0] == '<' && tmp[len(tmp)-1] == '>' && bytes.Index(tmp, []byte(`</`)) > 0
	if !isLikelyHTML {
		// not HTML, write back directly
		_, err := p.output.Write(buf)
		return err
	}

	// add our extra head HTML into output
	headBytes := []byte("<head>")
	posHead := bytes.Index(buf, headBytes)
	var part1, part2 []byte
	if posHead >= 0 {
		part1, part2 = buf[:posHead+len(headBytes)], buf[posHead+len(headBytes):]
	} else {
		part1, part2 = nil, buf
	}
	if len(part1) > 0 {
		if _, err := p.output.Write(part1); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(p.output, string(p.extraHeadHTML)); err != nil {
		return err
	}
	_, err := p.output.Write(part2)
	return err
}
