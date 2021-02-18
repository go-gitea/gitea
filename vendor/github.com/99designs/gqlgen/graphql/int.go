package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func MarshalInt(i int) Marshaler {
	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, strconv.Itoa(i))
	})
}

func UnmarshalInt(v interface{}) (int, error) {
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

func MarshalInt64(i int64) Marshaler {
	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, strconv.FormatInt(i, 10))
	})
}

func UnmarshalInt64(v interface{}) (int64, error) {
	switch v := v.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("%T is not an int", v)
	}
}

func MarshalInt32(i int32) Marshaler {
	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, strconv.FormatInt(int64(i), 10))
	})
}

func UnmarshalInt32(v interface{}) (int32, error) {
	switch v := v.(type) {
	case string:
		iv, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return 0, err
		}
		return int32(iv), nil
	case int:
		return int32(v), nil
	case int64:
		return int32(v), nil
	case json.Number:
		iv, err := strconv.ParseInt(string(v), 10, 32)
		if err != nil {
			return 0, err
		}
		return int32(iv), nil
	default:
		return 0, fmt.Errorf("%T is not an int", v)
	}
}
