// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"bytes"
	"io"
)

type finalProcessor struct {
	renderInternal *RenderInternal

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
	_, err := p.output.Write(buf)
	return err
}
