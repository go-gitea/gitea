package graphql

import (
	"errors"
	"io"
	"strconv"
	"time"
)

func MarshalTime(t time.Time) Marshaler {
	if t.IsZero() {
		return Null
	}

	return WriterFunc(func(w io.Writer) {
		io.WriteString(w, strconv.Quote(t.Format(time.RFC3339)))
	})
}

func UnmarshalTime(v interface{}) (time.Time, error) {
	if tmpStr, ok := v.(string); ok {
		return time.Parse(time.RFC3339, tmpStr)
	}
	return time.Time{}, errors.New("time should be RFC3339 formatted string")
}
