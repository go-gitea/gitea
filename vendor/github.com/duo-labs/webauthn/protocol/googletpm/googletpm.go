package googletpm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// From github.com/google/go-tpm
// Portions of existing package conflicted with existing build environment
// and only needed very small amount of code for pubarea and certinfo structs
// so copied them out to this package

// Supported Algorithms.
const (
	AlgUnknown   Algorithm = 0x0000
	AlgRSA       Algorithm = 0x0001
	AlgSHA1      Algorithm = 0x0004
	AlgAES       Algorithm = 0x0006
	AlgKeyedHash Algorithm = 0x0008
	AlgSHA256    Algorithm = 0x000B
	AlgSHA384    Algorithm = 0x000C
	AlgSHA512    Algorithm = 0x000D
	AlgNull      Algorithm = 0x0010
	AlgRSASSA    Algorithm = 0x0014
	AlgRSAES     Algorithm = 0x0015
	AlgRSAPSS    Algorithm = 0x0016
	AlgOAEP      Algorithm = 0x0017
	AlgECDSA     Algorithm = 0x0018
	AlgECDH      Algorithm = 0x0019
	AlgECDAA     Algorithm = 0x001A
	AlgKDF2      Algorithm = 0x0021
	AlgECC       Algorithm = 0x0023
	AlgCTR       Algorithm = 0x0040
	AlgOFB       Algorithm = 0x0041
	AlgCBC       Algorithm = 0x0042
	AlgCFB       Algorithm = 0x0043
	AlgECB       Algorithm = 0x0044
)

// UnpackBuf recursively unpacks types from a reader just as encoding/binary
// does under binary.BigEndian, but with one difference: it unpacks a byte
// slice by first reading an integer with lengthPrefixSize bytes, then reading
// that many bytes. It assumes that incoming values are pointers to values so
// that, e.g., underlying slices can be resized as needed.
func UnpackBuf(buf io.Reader, elts ...interface{}) error {
	for _, e := range elts {
		v := reflect.ValueOf(e)
		k := v.Kind()
		if k != reflect.Ptr {
			return fmt.Errorf("all values passed to Unpack must be pointers, got %v", k)
		}

		if v.IsNil() {
			return errors.New("can't fill a nil pointer")
		}

		iv := reflect.Indirect(v)
		switch iv.Kind() {
		case reflect.Struct:
			// Decompose the struct and copy over the values.
			for i := 0; i < iv.NumField(); i++ {
				if err := UnpackBuf(buf, iv.Field(i).Addr().Interface()); err != nil {
					return err
				}
			}
		case reflect.Slice:
			var size int
			_, isHandles := e.(*[]Handle)

			switch {
			// []Handle always uses 2-byte length, even with TPM 1.2.
			case isHandles:
				var tmpSize uint16
				if err := binary.Read(buf, binary.BigEndian, &tmpSize); err != nil {
					return err
				}
				size = int(tmpSize)
			// TPM 2.0
			case lengthPrefixSize == tpm20PrefixSize:
				var tmpSize uint16
				if err := binary.Read(buf, binary.BigEndian, &tmpSize); err != nil {
					return err
				}
				size = int(tmpSize)
			// TPM 1.2
			case lengthPrefixSize == tpm12PrefixSize:
				var tmpSize uint32
				if err := binary.Read(buf, binary.BigEndian, &tmpSize); err != nil {
					return err
				}
				size = int(tmpSize)
			default:
				return fmt.Errorf("lengthPrefixSize is %d, must be either 2 or 4", lengthPrefixSize)
			}

			// A zero size is used by the TPM to signal that certain elements
			// are not present.
			if size == 0 {
				continue
			}

			// Make len(e) match size exactly.
			switch b := e.(type) {
			case *[]byte:
				if len(*b) >= size {
					*b = (*b)[:size]
				} else {
					*b = append(*b, make([]byte, size-len(*b))...)
				}
			case *[]Handle:
				if len(*b) >= size {
					*b = (*b)[:size]
				} else {
					*b = append(*b, make([]Handle, size-len(*b))...)
				}
			default:
				return fmt.Errorf("can't fill pointer to %T, only []byte or []Handle slices", e)
			}

			if err := binary.Read(buf, binary.BigEndian, e); err != nil {
				return err
			}
		default:
			if err := binary.Read(buf, binary.BigEndian, e); err != nil {
				return err
			}
		}

	}

	return nil
}

// lengthPrefixSize is the size in bytes of length prefix for byte slices.
//
// In TPM 1.2 this is 4 bytes.
// In TPM 2.0 this is 2 bytes.
var lengthPrefixSize int

const (
	tpm12PrefixSize = 4
	tpm20PrefixSize = 2
)

// UseTPM20LengthPrefixSize makes Pack/Unpack use TPM 2.0 encoding for byte
// arrays.
func UseTPM20LengthPrefixSize() {
	lengthPrefixSize = tpm20PrefixSize
}
