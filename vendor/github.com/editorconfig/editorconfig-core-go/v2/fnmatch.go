package editorconfig

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var (
	// findLeftBrackets matches the opening left bracket {.
	findLeftBrackets = regexp.MustCompile(`(^|[^\\])\{`)
	// findDoubleLeftBrackets matches the duplicated opening left bracket {{.
	findDoubleLeftBrackets = regexp.MustCompile(`(^|[^\\])\{\{`)
	// findLeftBrackets matches the closing right bracket {.
	findRightBrackets = regexp.MustCompile(`(^|[^\\])\}`)
	// findDoubleRightBrackets matches the duplicated opening left bracket {{.
	findDoubleRightBrackets = regexp.MustCompile(`(^|[^\\])\}\}`)
	// findNumericRange matches a range of number, e.g. -2..5.
	findNumericRange = regexp.MustCompile(`^([+-]?\d+)\.\.([+-]?\d+)$`)
)

// FnmatchCase tests whether the name matches the given pattern case included.
func FnmatchCase(pattern, name string) (bool, error) {
	p := translate(pattern)

	r, err := regexp.Compile(fmt.Sprintf("^%s$", p))
	if err != nil {
		return false, fmt.Errorf("error compiling %q: %w", pattern, err)
	}

	return r.MatchString(name), nil
}

func translate(pattern string) string { // nolint: funlen,gocognit,gocyclo,cyclop
	index := 0
	pat := []rune(pattern)
	length := len(pat)

	result := strings.Builder{}

	braceLevel := 0
	isEscaped := false
	inBrackets := false

	// Double left and right is a hack to pass the core-test suite.
	left := len(findLeftBrackets.FindAllString(pattern, -1))
	doubleLeft := len(findDoubleLeftBrackets.FindAllString(pattern, -1))
	right := len(findRightBrackets.FindAllString(pattern, -1))
	doubleRight := len(findDoubleRightBrackets.FindAllString(pattern, -1))
	matchesBraces := left+doubleLeft == right+doubleRight
	pathSeparator := "/"

	if runtime.GOOS == "windows" {
		pathSeparator = regexp.QuoteMeta("\\")
	}

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
				result.WriteString(fmt.Sprintf("[^%s]*", pathSeparator))
			}
		case '/':
			p := index
			if p+2 < length && pat[p] == '*' && pat[p+1] == '*' && pat[p+2] == '/' {
				result.WriteString(fmt.Sprintf("(?:%s|%s.*%s)", pathSeparator, pathSeparator, pathSeparator))

				index += 3
			} else {
				result.WriteRune(r)
			}
		case '?':
			result.WriteString(fmt.Sprintf("[^%s]", pathSeparator))
		case '[':
			if inBrackets { // nolint: nestif
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
