package goorgeous

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// ExtractOrgHeaders finds and returns all of the headers
// from a bufio.Reader and returns them as their own byte slice
func ExtractOrgHeaders(r *bufio.Reader) (fm []byte, err error) {
	var out bytes.Buffer
	endOfHeaders := true
	for endOfHeaders {
		p, err := r.Peek(2)
		if err != nil {
			return nil, err
		}
		if !charMatches(p[0], '#') && !charMatches(p[1], '+') {
			endOfHeaders = false
			break
		}
		line, _, err := r.ReadLine()
		if err != nil {
			return nil, err
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

var reHeader = regexp.MustCompile(`^#\+(\w+?): (.*)`)

// OrgHeaders find all of the headers from a byte slice and returns
// them as a map of string interface
func OrgHeaders(input []byte) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	scanner := bufio.NewScanner(bytes.NewReader(input))

	for scanner.Scan() {
		data := scanner.Bytes()
		if !charMatches(data[0], '#') && !charMatches(data[1], '+') {
			return out, nil
		}
		matches := reHeader.FindSubmatch(data)

		if len(matches) < 3 {
			continue
		}

		key := string(matches[1])
		val := matches[2]
		switch {
		case strings.ToLower(key) == "tags" || strings.ToLower(key) == "categories" || strings.ToLower(key) == "aliases":
			bTags := bytes.Split(val, []byte(" "))
			tags := make([]string, len(bTags))
			for idx, tag := range bTags {
				tags[idx] = string(tag)
			}
			out[key] = tags
		default:
			out[key] = string(val)
		}

	}
	return out, nil

}
