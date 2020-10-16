package editorconfig

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// findLeftBrackets matches the opening left bracket {
	findLeftBrackets = regexp.MustCompile(`(^|[^\\])\{`)
	// findLeftBrackets matches the closing right bracket {
	findRightBrackets = regexp.MustCompile(`(^|[^\\])\}`)
	// findNumericRange matches a range of number, e.g. -2..5
	findNumericRange = regexp.MustCompile(`^([+-]?\d+)\.\.([+-]?\d+)$`)
)

// FnmatchCase tests whether the name matches the given pattern case included.
func FnmatchCase(pattern, name string) (bool, error) {
	p := translate(pattern)

	r, err := regexp.Compile(fmt.Sprintf("^%s$", p))
	if err != nil {
		return false, err
	}

	return r.MatchString(name), nil
}

func translate(pattern string) string { // nolint: gocyclo
	index := 0
	pat := []rune(pattern)
	length := len(pat)

	result := strings.Builder{}

	braceLevel := 0
	isEscaped := false
	inBrackets := false

	matchesBraces := len(findLeftBrackets.FindAllString(pattern, -1)) == len(findRightBrackets.FindAllString(pattern, -1))

	for index < length {
		r := pat[index]
		index++

		switch r {
		case '*':
			p := index
			if p < length && pat[p] == '*' {
				result.WriteString(".*")
				index++
			} else {
				result.WriteString("[^/]*")
			}
		case '/':
			p := index
			if p+2 < length && pat[p] == '*' && pat[p+1] == '*' && pat[p+2] == '/' {
				result.WriteString("(?:/|/.*/)")

				index += 3
			} else {
				result.WriteRune(r)
			}
		case '?':
			result.WriteString("[^/]")
		case '[':
			if inBrackets {
				result.WriteString("\\[")
			} else {
				hasSlash := false
				res := strings.Builder{}

				p := index
				for p < length {
					if pat[p] == ']' && pat[p-1] != '\\' {
						break
					}
					res.WriteRune(pat[p])
					if pat[p] == '/' && pat[p-1] != '\\' {
						hasSlash = true
						break
					}
					p++
				}
				if hasSlash {
					result.WriteString("\\[" + res.String())
					index = p + 1
				} else {
					inBrackets = true
					if index < length && pat[index] == '!' || pat[index] == '^' {
						index++
						result.WriteString("[^")
					} else {
						result.WriteRune('[')
					}
				}
			}
		case ']':
			if inBrackets && pat[index-2] == '\\' {
				result.WriteString("\\]")
			} else {
				result.WriteRune(r)
				inBrackets = false
			}
		case '{':
			hasComma := false
			p := index
			res := strings.Builder{}

			for p < length {
				if pat[p] == '}' && pat[p-1] != '\\' {
					break
				}

				res.WriteRune(pat[p])

				if pat[p] == ',' && pat[p-1] != '\\' {
					hasComma = true
					break
				}
				p++
			}

			switch {
			case !hasComma && p < length:
				inner := res.String()

				sub := findNumericRange.FindStringSubmatch(inner)
				if len(sub) == 3 {
					from, _ := strconv.Atoi(sub[1])
					to, _ := strconv.Atoi(sub[2])

					result.WriteString("(?:")

					// XXX does not scale well
					for i := from; i < to; i++ {
						result.WriteString(strconv.Itoa(i))
						result.WriteRune('|')
					}

					result.WriteString(strconv.Itoa(to))
					result.WriteRune(')')
				} else {
					r := translate(inner)

					result.WriteString(fmt.Sprintf("\\{%s\\}", r))
				}

				index = p + 1
			case matchesBraces:
				result.WriteString("(?:")
				braceLevel++
			default:
				result.WriteString("\\{")
			}
		case '}':
			if braceLevel > 0 {
				if isEscaped {
					result.WriteRune('}')

					isEscaped = false
				} else {
					result.WriteRune(')')

					braceLevel--
				}
			} else {
				result.WriteString("\\}")
			}
		case ',':
			if braceLevel == 0 || isEscaped {
				result.WriteRune(r)
			} else {
				result.WriteRune('|')
			}
		default:
			if r != '\\' || isEscaped {
				result.WriteString(regexp.QuoteMeta(string(r)))

				isEscaped = false
			} else {
				isEscaped = true
			}
		}
	}

	return result.String()
}
