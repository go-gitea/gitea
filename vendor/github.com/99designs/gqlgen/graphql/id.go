package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func MarshalID(s string) Marshaler {
	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, strconv.Quote(s))
	})
}
func UnmarshalID(v interface{}) (string, error) {
	switch v := v.(type) {
	case string:
		return v, nil
	case json.Number:
		return string(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
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

func MarshalIntID(i int) Marshaler {
	return WriterFunc(func(w io.Writer) {
		writeQuotedString(w, strconv.Itoa(i))
	})
}

func UnmarshalIntID(v interface{}) (int, error) {
	switch v := v.(type) {
	case string:
		return strconv.Atoi(v)
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case json.Number:
		return strconv.Atoi(string(v))
	default:
		return 0, fmt.Errorf("%T is not an int", v)
	}
}
