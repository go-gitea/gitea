// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package foreachref

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Parser parses 'git for-each-ref' output according to a given output Format.
type Parser struct {
	//  tokenizes 'git for-each-ref' output into "reference paragraphs".
	scanner *bufio.Scanner

	// format represents the '--format' string that describes the expected
	// 'git for-each-ref' output structure.
	format Format

	// err holds the last encountered error during parsing.
	err error
}

// NewParser creates a 'git for-each-ref' output parser that will parse all
// references in the provided Reader. The references in the output are assumed
// to follow the specified Format.
func NewParser(r io.Reader, format Format) *Parser {
	scanner := bufio.NewScanner(r)

	// default MaxScanTokenSize = 64 kiB may be too small for some references,
	// so allow the buffer to grow up to 4x if needed
	scanner.Buffer(nil, 4*bufio.MaxScanTokenSize)

	// in addition to the reference delimiter we specified in the --format,
	// `git for-each-ref` will always add a newline after every reference.
	refDelim := make([]byte, 0, len(format.refDelim)+1)
	refDelim = append(refDelim, format.refDelim...)
	refDelim = append(refDelim, '\n')

	// Split input into delimiter-separated "reference blocks".
	scanner.Split(
		func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			// Scan until delimiter, marking end of reference.
			delimIdx := bytes.Index(data, refDelim)
			if delimIdx >= 0 {
				token := data[:delimIdx]
				advance := delimIdx + len(refDelim)
				return advance, token, nil
			}
			// If we're at EOF, we have a final, non-terminated reference. Return it.
			if atEOF {
				return len(data), data, nil
			}
			// Not yet a full field. Request more data.
			return 0, nil, nil
		})

	return &Parser{
		scanner: scanner,
		format:  format,
		err:     nil,
	}
}

// Next returns the next reference as a collection of key-value pairs. nil
// denotes EOF but is also returned on errors. The Err method should always be
// consulted after Next returning nil.
//
// It could, for example return something like:
//
//	{ "objecttype": "tag", "refname:short": "v1.16.4", "object": "f460b7543ed500e49c133c2cd85c8c55ee9dbe27" }
func (p *Parser) Next() map[string]string {
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			p.err = err
		}
		return nil
	}
	fields, err := p.parseRef(p.scanner.Text())
	if err != nil {
		p.err = err
		return nil
	}
	return fields
}

// Err returns the latest encountered parsing error.
func (p *Parser) Err() error {
	return p.err
}

// parseRef parses out all key-value pairs from a single reference block, such as
//
//	"objecttype tag\0refname:short v1.16.4\0object f460b7543ed500e49c133c2cd85c8c55ee9dbe27"
func (p *Parser) parseRef(refBlock string) (map[string]string, error) {
	if refBlock == "" {
		// must be at EOF
		return nil, nil
	}

	fieldValues := make(map[string]string)

	fields := strings.Split(refBlock, p.format.fieldDelimStr)
	if len(fields) != len(p.format.fieldNames) {
		return nil, fmt.Errorf("unexpected number of reference fields: wanted %d, was %d",
			len(fields), len(p.format.fieldNames))
	}
	for i, field := range fields {
		field = strings.TrimSpace(field)

		var fieldKey string
		var fieldVal string
		firstSpace := strings.Index(field, " ")
		if firstSpace > 0 {
			fieldKey = field[:firstSpace]
			fieldVal = field[firstSpace+1:]
		} else {
			// could be the case if the requested field had no value
			fieldKey = field
		}

		// enforce the format order of fields
		if p.format.fieldNames[i] != fieldKey {
			return nil, fmt.Errorf("unexpected field name at position %d: wanted: '%s', was: '%s'",
				i, p.format.fieldNames[i], fieldKey)
		}

		fieldValues[fieldKey] = fieldVal
	}

	return fieldValues, nil
}
