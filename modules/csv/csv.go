// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"io"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
)

const (
	maxLines        = 10
	guessSampleSize = 1e4 // 10k
)

// CreateReader creates a csv.Reader with the given delimiter.
func CreateReader(input io.Reader, delimiter rune) *stdcsv.Reader {
	rd := stdcsv.NewReader(input)
	rd.Comma = delimiter
	if delimiter != '\t' && delimiter != ' ' {
		// TrimLeadingSpace can't be true when delimiter is a tab or a space as the value for a column might be empty,
		// thus would change `\t\t` to just `\t` or `  ` (two spaces) to just ` ` (single space)
		rd.TrimLeadingSpace = true
	}
	return rd
}

// CreateReaderAndDetermineDelimiter tries to guess the field delimiter from the content and creates a csv.Reader.
// Reads at most guessSampleSize bytes.
func CreateReaderAndDetermineDelimiter(ctx *markup.RenderContext, rd io.Reader) (*stdcsv.Reader, error) {
	data := make([]byte, guessSampleSize)
	size, err := util.ReadAtMost(rd, data)
	if err != nil {
		return nil, err
	}

	return CreateReader(
		io.MultiReader(bytes.NewReader(data[:size]), rd),
		determineDelimiter(ctx, data[:size]),
	), nil
}

// determineDelimiter takes a RenderContext and if it isn't nil and the Filename has an extension that specifies the delimiter,
// it is used as the delimiter. Otherwise we call guessDelimiter with the data passed
func determineDelimiter(ctx *markup.RenderContext, data []byte) rune {
	extension := ".csv"
	if ctx != nil {
		extension = strings.ToLower(path.Ext(ctx.RenderOptions.RelativePath))
	}

	var delimiter rune
	switch extension {
	case ".tsv":
		delimiter = '\t'
	case ".psv":
		delimiter = '|'
	default:
		delimiter = guessDelimiter(data)
	}

	return delimiter
}

// quoteRegexp follows the RFC-4180 CSV standard for when double-quotes are used to enclose fields, then a double-quote appearing inside a
// field must be escaped by preceding it with another double quote. https://www.ietf.org/rfc/rfc4180.txt
// This finds all quoted strings that have escaped quotes.
var quoteRegexp = regexp.MustCompile(`"[^"]*"`)

// removeQuotedStrings uses the quoteRegexp to remove all quoted strings so that we can reliably have each row on one line
// (quoted strings often have new lines within the string)
func removeQuotedString(text string) string {
	return quoteRegexp.ReplaceAllLiteralString(text, "")
}

// guessDelimiter takes up to maxLines of the CSV text, iterates through the possible delimiters, and sees if the CSV Reader reads it without throwing any errors.
// If more than one delimiter passes, the delimiter that results in the most columns is returned.
func guessDelimiter(data []byte) rune {
	delimiter := guessFromBeforeAfterQuotes(data)
	if delimiter != 0 {
		return delimiter
	}

	// Removes quoted values so we don't have columns with new lines in them
	text := removeQuotedString(string(data))

	// Make the text just be maxLines or less, ignoring truncated lines
	lines := strings.SplitN(text, "\n", maxLines+1) // Will contain at least one line, and if there are more than MaxLines, the last item holds the rest of the lines
	if len(lines) > maxLines {
		// If the length of lines is > maxLines we know we have the max number of lines, trim it to maxLines
		lines = lines[:maxLines]
	} else if len(lines) > 1 && len(data) >= guessSampleSize {
		// Even with data >= guessSampleSize, we don't have maxLines + 1 (no extra lines, must have really long lines)
		// thus the last line is probably have a truncated line. Drop the last line if len(lines) > 1
		lines = lines[:len(lines)-1]
	}

	// Put lines back together as a string
	text = strings.Join(lines, "\n")

	delimiters := []rune{',', '\t', ';', '|', '@'}
	validDelim := delimiters[0]
	validDelimColCount := 0
	for _, delim := range delimiters {
		csvReader := stdcsv.NewReader(strings.NewReader(text))
		csvReader.Comma = delim
		if rows, err := csvReader.ReadAll(); err == nil && len(rows) > 0 && len(rows[0]) > validDelimColCount {
			validDelim = delim
			validDelimColCount = len(rows[0])
		}
	}
	return validDelim
}

// FormatError converts csv errors into readable messages.
func FormatError(err error, locale translation.Locale) (string, error) {
	if perr, ok := err.(*stdcsv.ParseError); ok {
		if perr.Err == stdcsv.ErrFieldCount {
			return locale.TrString("repo.error.csv.invalid_field_count", perr.Line), nil
		}
		return locale.TrString("repo.error.csv.unexpected", perr.Line, perr.Column), nil
	}

	return "", err
}

// Looks for possible delimiters right before or after (with spaces after the former) double quotes with closing quotes
var beforeAfterQuotes = regexp.MustCompile(`([,@\t;|]{0,1}) *(?:"[^"]*")+([,@\t;|]{0,1})`)

// guessFromBeforeAfterQuotes guesses the limiter by finding a double quote that has a valid delimiter before it and a closing quote,
// or a double quote with a closing quote and a valid delimiter after it
func guessFromBeforeAfterQuotes(data []byte) rune {
	rs := beforeAfterQuotes.FindStringSubmatch(string(data)) // returns first match, or nil if none
	if rs != nil {
		if rs[1] != "" {
			return rune(rs[1][0]) // delimiter found left of quoted string
		} else if rs[2] != "" {
			return rune(rs[2][0]) // delimiter found right of quoted string
		}
	}
	return 0 // no match found
}
