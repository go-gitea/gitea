// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"encoding/csv"
	"html"
	"io"
	"math"
	"strings"

	"code.gitea.io/gitea/modules/markup"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for orgmode
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "csv"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".csv", ".tsv"}
}

// Render implements markup.Parser
func (p Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	rd := csv.NewReader(bytes.NewReader(rawBytes))
	rd.Comma = p.bestDelimiter(rawBytes)
	var tmpBlock bytes.Buffer
	tmpBlock.WriteString(`<table class="table">`)
	for {
		fields, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		tmpBlock.WriteString("<tr>")
		for _, field := range fields {
			tmpBlock.WriteString("<td>")
			tmpBlock.WriteString(html.EscapeString(field))
			tmpBlock.WriteString("</td>")
		}
		tmpBlock.WriteString("<tr>")
	}
	tmpBlock.WriteString("</table>")

	return tmpBlock.Bytes()
}

func (p Parser) bestDelimiter(data []byte) rune {
	// Scores the input data against delimiters, and returns the best matching.
	// Reads at most 10k bytes & 10 lines.
	maxLines := 10
	maxBytes := int(math.Min(float64(len(data)), 1e4))
	text := string(data[:maxBytes])
	lines := strings.SplitN(text, "\n", maxLines+1)[:maxLines]

	delimiters := []rune{',', ';', '\t', '|'}
	bestDelim := delimiters[0]
	bestScore := 0.0

	for _, delim := range delimiters {
		score := p.scoreDelimiter(lines, delim)
		if score > bestScore {
			bestScore = score
			bestDelim = delim
		}
	}

	return bestDelim
}

func (Parser) scoreDelimiter(lines []string, delim rune) (score float64) {
	// Scores a delimiter against input csv data with a count and regularity metric.

	countTotal := 0.0
	countLineMax := 0.0
	linesNotEqual := 0.0

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		countLine := float64(strings.Count(line, string(delim)))
		countTotal += countLine

		if countLine != countLineMax {
			if countLineMax != 0 {
				linesNotEqual++
			}
			countLineMax = math.Max(countLine, countLineMax)
		}
	}

	return countTotal * (1 - linesNotEqual/float64(len(lines)))
}
