package graphql

import (
	"fmt"
	"io"
	"strconv"
)

const encodeHex = "0123456789ABCDEF"

func MarshalString(s string) Marshaler {
	return WriterFunc(func(w io.Writer) {
		writeQuotedString(w, s)
	})
}

func writeQuotedString(w io.Writer, s string) {
	start := 0
	io.WriteString(w, `"`)

	for i, c := range s {
		if c < 0x20 || c == '\\' || c == '"' {
			io.WriteString(w, s[start:i])

			switch c {
			case '\t':
				io.WriteString(w, `\t`)
			case '\r':
				io.WriteString(w, `\r`)
			case '\n':
				io.WriteString(w, `\n`)
			case '\\':
				io.WriteString(w, `\\`)
			case '"':
				io.WriteString(w, `\"`)
			default:
				io.WriteString(w, `\u00`)
				w.Write([]byte{encodeHex[c>>4], encodeHex[c&0xf]})
			}

			start = i + 1
		}
	}

	io.WriteString(w, s[start:])
	io.WriteString(w, `"`)
}

func UnmarshalString(v interface{}) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case float64:
		return fmt.Sprintf("%f", v), nil
	case bool:
		if v {
			return "true", nil
		} else {
			return "false", nil
		}
	case nil:
		return "null", nil
	default:
		return "", fmt.Errorf("%T is not a string", v)
	}
}
