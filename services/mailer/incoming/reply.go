// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/util"
)

const (
	yearToken = `\b\d{4}\b`            // 4-digit year
	timeToken = `\b\d{1,2}[:.]\d{2}\b` // HH:MM or HH.MM
	// "wrote" verbs ending an attribution line; CJK ones are matched without a
	// preceding word-separator since those scripts don't space their words
	wroteVerbs    = `wrote|writes|schrieb|skrev|napisał|escreveu|escribió|написал|пише|a écrit`
	cjkWroteVerbs = `写道|寫道|書きました|작성`
	// device names anchoring CJK mobile signatures, so prose isn't mistaken for one
	cjkDevice = `iphone|ipad|ipod|android|galaxy|手机|手機|平板`
)

// forwarded-mail header fields across the common mail clients/locales. headerFromFields
// (the "From"-equivalents) must begin a block; headerFields is the full set allowed to
// follow. Matched as a prefix by headerLine, so adding a locale is a one-line change.
var (
	headerFromFields = []string{
		"from", "fra", "de", "von", "da", "van", "från", "expéditeur",
		"发件人", "寄件者", "差出人", "보낸사람",
	}
	headerFields = append([]string{
		"to", "cc", "bcc", "sent", "date", "subject", "reply-to",
		"til", "emne", "an", "betreff", "gesendet", "para", "assunto", "asunto",
		"risposta", "inviato", "oggetto", "destinataire", "objet", "répondre à",
		"aan", "onderwerp", "beantwoorden", "skickat", "till", "ämne",
		"收件人", "主题", "主旨", "主題", "收件者", "抄送", "日期", "宛先", "件名", "받는사람", "제목",
	}, headerFromFields...)
)

// patterns are compiled on first use so the incoming-mail feature adds nothing to startup.
var patterns = sync.OnceValue(func() (ret struct {
	signature, attribution, separator *regexp.Regexp
},
) {
	// "-- " delimiter and common mobile footers with frequent localizations. The CJK
	// forms require a device name so ordinary prose like "发自我的内心" or "会議から送信"
	// is not mistaken for a signature.
	ret.signature = regexp.MustCompile(`(?i)^(--|__|—` +
		`|sent (from|via|with) .+|get outlook for .+` +
		`|envoyé depuis mon .+|sendt fra min .+|von meinem .+|verzonden (met|vanaf) .+` +
		`|(發|发)自我的.*(` + cjkDevice + `).*` +
		`|.*(` + cjkDevice + `).*(から送信|에서 보냄|傳送|发送))$`)

	// attribution introducing quoted history: a line ending in a "wrote:" verb
	// (Latin/Cyrillic or CJK), a "Name <email> wrote" line, a lead word directly
	// followed by a day number or weekday plus a year and a time, or an ISO-date-led
	// line. The date phrasing, trailing colon and the email before the verb guard
	// against prose (so "On the 2024 roadmap … at 10:00" is not an attribution).
	ret.attribution = regexp.MustCompile(`(?i)^>*\s*(` +
		`.*[\s">'](` + wroteVerbs + `)\s*[:：]` +
		`|.*(` + cjkWroteVerbs + `)\s*[:：]` +
		`|.*<\S+@\S+>\s+(` + wroteVerbs + `)\b.*` +
		`|(on|at|le|am|el|em|den|il|op|dnia|w dniu)\b[\s,]*(\d|(?:mon|tue|wed|thu|fri|sat|sun)\b).*` + yearToken + `.*` + timeToken + `.*` +
		`|\d{4}-\d{2}-\d{2}\b.*` + timeToken + `.*` +
		`)$`)

	// a dash/underscore rule line, or text fenced by dashes such as
	// "-------- Original Message --------" or "-----Mensaje original-----"
	ret.separator = regexp.MustCompile(`(?i)^\s*\*?\s*([-_]{5,}|-{2,}.+-{2,}|original message|forwarded message)\s*\*?\s*$`)
	return ret
})

// extractReply returns the user-written part of a plain-text email body, dropping
// quoted history, the reply attribution, signatures and forwarded headers. It is a
// slim, dependency-free reimplementation based on github.com/dimiro1/reply (MIT),
// covering the common mail-client formats and languages; bottom posting and
// forwarded bodies are not handled.
func extractReply(text string) string {
	p := patterns()
	lines := strings.Split(util.NormalizeStringEOL(text), "\n")

	// cut at the first line that begins quoted history, a signature or a header block
	for i := range lines {
		trimmed := strings.TrimSpace(lines[i])
		if p.signature.MatchString(trimmed) || p.attribution.MatchString(trimmed) ||
			p.separator.MatchString(trimmed) || headerBlock(trimmed, lines[i+1:]) {
			lines = lines[:i]
			break
		}
	}

	// drop the trailing block of quoted/blank lines, unless the whole body is quoted
	end := len(lines)
	for end > 0 {
		// "ᐧ" is the trailing marker some mobile clients (Mailbox) append
		if t := strings.TrimSpace(lines[end-1]); t != "" && t != "ᐧ" && !strings.HasPrefix(t, ">") {
			break
		}
		end--
	}
	if end > 0 {
		lines = lines[:end]
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// headerBlock reports whether a forwarded-mail header block starts here: the
// (already-trimmed) first line is a "From" field and the next non-blank line is
// another field, so a lone "Subject:" sentence is not a boundary.
func headerBlock(first string, rest []string) bool {
	if !headerLine(first, headerFromFields) {
		return false
	}
	for _, next := range rest {
		if t := strings.TrimSpace(next); t != "" {
			return headerLine(t, headerFields)
		}
	}
	return false
}

// headerLine reports whether the already-trimmed line is a "Field:" header for one
// of fields. An ASCII colon must be followed by a space so prose like "To:do this"
// is ignored; the CJK fullwidth colon "：" needs no space.
func headerLine(line string, fields []string) bool {
	lower := strings.ToLower(line)
	for _, field := range fields {
		if rest, ok := strings.CutPrefix(lower, field); ok &&
			(strings.HasPrefix(rest, ": ") || strings.HasPrefix(rest, "：")) {
			return true
		}
	}
	return false
}
