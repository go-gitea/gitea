package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func MarshalFloat(f float64) Marshaler {
	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, fmt.Sprintf("%g", f))
	})
}

func UnmarshalFloat(v interface{}) (float64, error) {
	switch v := v.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case json.Number:
		return strconv.ParseFloat(string(v), 64)
	default:
		return 0, fmt.Errorf("%T is not an float", v)
	}
}
