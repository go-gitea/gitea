package protocol

import (
	"bytes"
	"encoding/base64"
	"reflect"
)

// URLEncodedBase64 represents a byte slice holding URL-encoded base64 data.
// When fields of this type are unmarshaled from JSON, the data is base64
// decoded into a byte slice.
type URLEncodedBase64 []byte

// UnmarshalJSON base64 decodes a URL-encoded value, storing the result in the
// provided byte slice.
func (dest *URLEncodedBase64) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	// Trim the leading spaces
	data = bytes.Trim(data, "\"")
	out := make([]byte, base64.RawURLEncoding.DecodedLen(len(data)))
	n, err := base64.RawURLEncoding.Decode(out, data)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(dest).Elem()
	v.SetBytes(out[:n])
	return nil
}

// MarshalJSON base64 encodes a non URL-encoded value, storing the result in the
// provided byte slice.
func (data URLEncodedBase64) MarshalJSON() ([]byte, error) {
	if data == nil {
		return []byte("null"), nil
	}
	return []byte(`"` + base64.RawURLEncoding.EncodeToString(data) + `"`), nil
}
