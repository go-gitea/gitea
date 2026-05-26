// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"regexp"
	"strings"
)

const (
	yearToken = `\b\d{4}\b`            // 4-digit year
	timeToken = `\b\d{1,2}[:.]\d{2}\b` // HH:MM or HH.MM
)

var (
	// "-- " delimiter and common mobile footers with frequent localizations
	signatureRegex = regexp.MustCompile(`(?i)^(--|__|—` +
		`|sent (from|via|with) .+|get outlook for .+` +
		`|envoyé depuis mon .+|sendt fra min .+|von meinem .+|verzonden (met|vanaf) .+)$`)

	// attribution introducing quoted history: a line ending in a "wrote:" verb,
	// a lead word followed by both a date and a time, or an ISO-date-led line.
	// The date+time and trailing colon guard against ordinary prose matching.
	attributionRegex = regexp.MustCompile(`(?i)^>*\s*(` +
		`.*[\s">'](wrote|writes|schrieb|skrev|napisał|escreveu|escribió|написал|пише|a écrit)\s*[:：]` +
		`|(on|at|le|am|el|em|den|il|op|dnia|w dniu)\b.*` + yearToken + `.*` + timeToken + `.*` +
		`|\d{4}-\d{2}-\d{2}\b.*` + timeToken + `.*` +
		`)$`)

	// a dash/underscore rule line, or text fenced by dashes such as
	// "-------- Original Message --------" or "-----Mensaje original-----"
	separatorRegex = regexp.MustCompile(`(?i)^\s*\*?\s*([-_]{5,}|-{2,}.+-{2,}|original message|forwarded message)\s*\*?\s*$`)

	// a forwarded-mail header block starts with a "From" field (English plus
	// common localizations); headerBlock requires a second field to follow
	headerStartRegex = regexp.MustCompile(`(?i)^(from|von|från|da|de):\s`)
	headerFieldRegex = regexp.MustCompile(`(?i)^(from|to|cc|bcc|sent|date|subject|reply-to` +
		`|von|gesendet|an|betreff|från|skickat|till|ämne|da|risposta|inviato|oggetto):\s`)

	quoteRegex = regexp.MustCompile(`^\s*>`)
)

// extractReply returns the user-written part of a plain-text email body, dropping
// quoted history, the reply attribution, signatures and forwarded headers. It is a
// slim, dependency-free replacement for github.com/dimiro1/reply covering the common
// mail-client formats and languages; bottom posting and forwarded bodies are not handled.
func extractReply(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	// cut at the first line that begins quoted history, a signature or a header block
	for i := range lines {
		if isBoundary(lines[i:]) {
			lines = lines[:i]
			break
		}
	}

	// drop the trailing block of quoted/blank lines, unless the whole body is quoted
	end := len(lines)
	for end > 0 && (strings.TrimSpace(lines[end-1]) == "" || quoteRegex.MatchString(lines[end-1])) {
		end--
	}
	if end > 0 {
		lines = lines[:end]
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isBoundary(lines []string) bool {
	trimmed := strings.TrimSpace(lines[0])
	return signatureRegex.MatchString(trimmed) ||
		attributionRegex.MatchString(trimmed) ||
		separatorRegex.MatchString(trimmed) ||
		headerBlock(lines)
}

// headerBlock reports whether lines start a forwarded-mail header block: a "From"
// field followed by another field, so a lone "Subject:" sentence is not a boundary.
func headerBlock(lines []string) bool {
	if !headerStartRegex.MatchString(lines[0]) {
		return false
	}
	for _, next := range lines[1:] {
		if strings.TrimSpace(next) != "" {
			return headerFieldRegex.MatchString(next)
		}
	}
	return false
}
