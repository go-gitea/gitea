package websspi

import (
	"unicode/utf16"
	"unsafe"
)

// UTF16PtrToString converts a pointer to a UTF16 string to a string
func UTF16PtrToString(ptr *uint16, maxLen int) (s string) {
	if ptr == nil {
		return ""
	}
	buf := make([]uint16, 0, maxLen)
	for i, p := 0, uintptr(unsafe.Pointer(ptr)); i < maxLen; i, p = i+1, p+2 {
		char := *(*uint16)(unsafe.Pointer(p))
		if char == 0 {
			return string(utf16.Decode(buf))
		}
		buf = append(buf, char)
	}
	return ""
}
