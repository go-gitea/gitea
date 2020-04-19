package xml

var (
	ltEntityBytes          = []byte("&lt;")
	ampEntityBytes         = []byte("&amp;")
	singleQuoteEntityBytes = []byte("&#39;")
	doubleQuoteEntityBytes = []byte("&#34;")
)

// Entities are all named character entities.
var EntitiesMap = map[string][]byte{
	"apos": []byte("'"),
	"gt":   []byte(">"),
	"quot": []byte("\""),
}

// EscapeAttrVal returns the escape attribute value bytes without quotes.
func EscapeAttrVal(buf *[]byte, b []byte) []byte {
	singles := 0
	doubles := 0
	for _, c := range b {
		if c == '"' {
			doubles++
		} else if c == '\'' {
			singles++
		}
	}

	n := len(b) + 2
	var quote byte
	var escapedQuote []byte
	if doubles > singles {
		n += singles * 4
		quote = '\''
		escapedQuote = singleQuoteEntityBytes
	} else {
		n += doubles * 4
		quote = '"'
		escapedQuote = doubleQuoteEntityBytes
	}
	if n > cap(*buf) {
		*buf = make([]byte, 0, n) // maximum size, not actual size
	}
	t := (*buf)[:n] // maximum size, not actual size
	t[0] = quote
	j := 1
	start := 0
	for i, c := range b {
		if c == quote {
			j += copy(t[j:], b[start:i])
			j += copy(t[j:], escapedQuote)
			start = i + 1
		}
	}
	j += copy(t[j:], b[start:])
	t[j] = quote
	return t[:j+1]
}

// EscapeCDATAVal returns the escaped text bytes.
func EscapeCDATAVal(buf *[]byte, b []byte) ([]byte, bool) {
	n := 0
	for _, c := range b {
		if c == '<' || c == '&' {
			if c == '<' {
				n += 3 // &lt;
			} else {
				n += 4 // &amp;
			}
			if n > len("<![CDATA[]]>") {
				return b, false
			}
		}
	}
	if len(b)+n > cap(*buf) {
		*buf = make([]byte, 0, len(b)+n)
	}
	t := (*buf)[:len(b)+n]
	j := 0
	start := 0
	for i, c := range b {
		if c == '<' {
			j += copy(t[j:], b[start:i])
			j += copy(t[j:], ltEntityBytes)
			start = i + 1
		} else if c == '&' {
			j += copy(t[j:], b[start:i])
			j += copy(t[j:], ampEntityBytes)
			start = i + 1
		}
	}
	j += copy(t[j:], b[start:])
	return t[:j], true
}
