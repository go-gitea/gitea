// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"errors"
	"io"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
)

var quoteRegexp = regexp.MustCompile(`["'][\s\S]+?["']`)

// CreateReader creates a csv.Reader with the given delimiter.
func CreateReader(input io.Reader, delimiter rune) *stdcsv.Reader {
	rd := stdcsv.NewReader(input)
	rd.Comma = delimiter
	rd.TrimLeadingSpace = true
	return rd
}

// CreateReaderAndGuessDelimiter tries to guess the field delimiter from the content and creates a csv.Reader.
// Reads at most 10k bytes.
func CreateReaderAndGuessDelimiter(rd io.Reader) (*stdcsv.Reader, error) {
	var data = make([]byte, 1e4)
	size, err := util.ReadAtMost(rd, data)
	if err != nil {
		return nil, err
	}

	return CreateReader(
		io.MultiReader(bytes.NewReader(data[:size]), rd),
		guessDelimiter(data[:size]),
	), nil
}

// guessDelimiter scores the input CSV data against delimiters, and returns the best match.
func guessDelimiter(data []byte) rune {
	maxLines := 10
	text := quoteRegexp.ReplaceAllLiteralString(string(data), "")
	lines := strings.SplitN(text, "\n", maxLines+1)
	lines = lines[:util.Min(maxLines, len(lines))]

	delimiters := []rune{',', ';', '\t', '|', '@'}
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

// scoreDelimiter uses a count & regularity metric to evaluate a delimiter against lines of CSV.
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

// FormatError converts csv errors into readable messages.
func FormatError(err error, locale translation.Locale) (string, error) {
	var perr *stdcsv.ParseError
	if errors.As(err, &perr) {
		if perr.Err == stdcsv.ErrFieldCount {
			return locale.Tr("repo.error.csv.invalid_field_count", perr.Line), nil
		}
		return locale.Tr("repo.error.csv.unexpected", perr.Line, perr.Column), nil
	}

	return "", err
}
