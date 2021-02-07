// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

var quoteRegexp = regexp.MustCompile(`["'][\s\S]+?["']`)

// CreateCsvReader creates a CSV reader with the given delimiter.
func CreateCsvReader(rawBytes []byte, delimiter rune) *csv.Reader {
	rd := csv.NewReader(bytes.NewReader(rawBytes))
	rd.Comma = delimiter
	return rd
}

// CreateCsvReaderAndGuessDelimiter creates a CSV reader with a guessed delimiter.
func CreateCsvReaderAndGuessDelimiter(rawBytes []byte) *csv.Reader {
	delimiter := guessDelimiter(rawBytes)
	return CreateCsvReader(rawBytes, delimiter)
}

// guessDelimiter scores the input CSV data against delimiters, and returns the best match.
// Reads at most 10k bytes & 10 lines.
func guessDelimiter(data []byte) rune {
	maxLines := 10
	maxBytes := util.Min(len(data), 1e4)
	text := string(data[:maxBytes])
	text = quoteRegexp.ReplaceAllLiteralString(text, "")
	lines := strings.SplitN(text, "\n", maxLines+1)
	lines = lines[:util.Min(maxLines, len(lines))]

	delimiters := []rune{',', ';', '\t', '|'}
	bestDelim := delimiters[0]
	bestScore := 0.0
	for _, delim := range delimiters {
		score := scoreDelimiter(lines, delim)
		if score > bestScore {
			bestScore = score
			bestDelim = delim
		}
	}

	return bestDelim
}

// scoreDelimiter uses a count & regularity metric to evaluate a delimiter against lines of CSV
func scoreDelimiter(lines []string, delim rune) float64 {
	countTotal := 0
	countLineMax := 0
	linesNotEqual := 0

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		countLine := strings.Count(line, string(delim))
		countTotal += countLine
		if countLine != countLineMax {
			if countLineMax != 0 {
				linesNotEqual++
			}
			countLineMax = util.Max(countLine, countLineMax)
		}
	}

	return float64(countTotal) * (1 - float64(linesNotEqual)/float64(len(lines)))
}
