package testfixtures

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func (l *Loader) tryHexStringToBytes(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") {
		return nil, fmt.Errorf("not a hexadecimal string, must be prefix 0x")
	}
	return hex.DecodeString(strings.TrimPrefix(s, "0x"))
}
