package graphql

import (
	"fmt"
	"io"
	"strings"
)

func MarshalBoolean(b bool) Marshaler {
	return WriterFunc(func(w io.Writer) {
		if b {
			w.Write(trueLit)
		} else {
			w.Write(falseLit)
		}
	})
}

func UnmarshalBoolean(v interface{}) (bool, error) {
	switch v := v.(type) {
	case string:
		return strings.ToLower(v) == "true", nil
	case int:
		return v != 0, nil
	case bool:
		return v, nil
	default:
		return false, fmt.Errorf("%T is not a bool", v)
	}
}
