package location

import (
	"regexp"

	"github.com/graphql-go/graphql/language/source"
)

type SourceLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func GetLocation(s *source.Source, position int) SourceLocation {
	body := []byte{}
	if s != nil {
		body = s.Body
	}
	line := 1
	column := position + 1
	lineRegexp := regexp.MustCompile("\r\n|[\n\r]")
	matches := lineRegexp.FindAllIndex(body, -1)
	for _, match := range matches {
		matchIndex := match[0]
		if matchIndex < position {
			line++
			l := len(s.Body[match[0]:match[1]])
			column = position + 1 - (matchIndex + l)
			continue
		} else {
			break
		}
	}
	return SourceLocation{Line: line, Column: column}
}
