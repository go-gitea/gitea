// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/setting"
)

func buildIssuesCalendarUID(issueID int64) string {
	host := setting.Domain
	if host == "" {
		if parsed, err := url.Parse(setting.AppURL); err == nil {
			host = parsed.Host
		}
	}
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("issue-%d@%s", issueID, host)
}

func escapeCalendarText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
		"\r", "\\n",
	)
	return replacer.Replace(value)
}

// BuildIssuesCalendar creates an iCalendar payload for issues that have due dates.
func BuildIssuesCalendar(ctx context.Context, issues issues_model.IssueList, calendarName string) ([]byte, error) {
	var buf bytes.Buffer
	now := time.Now().UTC()
	writeLine := func(line string) {
		buf.WriteString(line)
		buf.WriteString("\r\n")
	}

	writeLine("BEGIN:VCALENDAR")
	writeLine("VERSION:2.0")
	writeLine("PRODID:-//Gitea//NONSGML Gitea//EN")
	writeLine("CALSCALE:GREGORIAN")
	if calendarName != "" {
		writeLine("X-WR-CALNAME:" + escapeCalendarText(calendarName))
	}

	for _, issue := range issues {
		if issue.DeadlineUnix.IsZero() {
			continue
		}
		if err := issue.LoadRepo(ctx); err != nil {
			return nil, err
		}

		issueURL := issue.HTMLURL(ctx)
		summary := fmt.Sprintf("%s (%s)", issue.Title, issue.Repo.FullName())
		description := "Find out more at " + issueURL

		writeLine("BEGIN:VEVENT")
		writeLine("DTSTAMP:" + now.Format("20060102T150405Z"))
		writeLine("UID:" + buildIssuesCalendarUID(issue.ID))
		writeLine("DTSTART;VALUE=DATE:" + issue.DeadlineUnix.AsTime().Format("20060102"))
		writeLine("SUMMARY:" + escapeCalendarText(summary))
		writeLine("DESCRIPTION:" + escapeCalendarText(description))
		writeLine("TRANSP:TRANSPARENT")
		writeLine("URL:" + escapeCalendarText(issueURL))
		writeLine("END:VEVENT")
	}

	writeLine("END:VCALENDAR")
	return buf.Bytes(), nil
}
