package qr

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/boombuler/barcode/utils"
)

func encodeNumeric(content string, ecl ErrorCorrectionLevel) (*utils.BitList, *versionInfo, error) {
	contentBitCount := (len(content) / 3) * 10
	switch len(content) % 3 {
	case 1:
		contentBitCount += 4
	case 2:
		contentBitCount += 7
	}
	vi := findSmallestVersionInfo(ecl, numericMode, contentBitCount)
	if vi == nil {
		return nil, nil, errors.New("To much data to encode")
	}
	res := new(utils.BitList)
	res.AddBits(int(numericMode), 4)
	res.AddBits(len(content), vi.charCountBits(numericMode))

	for pos := 0; pos < len(content); pos += 3 {
		var curStr string
		if pos+3 <= len(content) {
			curStr = content[pos : pos+3]
		} else {
			curStr = content[pos:]
		}

		i, err := strconv.Atoi(curStr)
		if err != nil || i < 0 {
			return nil, nil, fmt.Errorf("\"%s\" can not be encoded as %s", content, Numeric)
		}
		var bitCnt byte
		switch len(curStr) % 3 {
		case 0:
			bitCnt = 10
		case 1:
			bitCnt = 4
			break
		case 2:
			bitCnt = 7
			break
		}

		res.AddBits(i, bitCnt)
	}

	addPaddingAndTerminator(res, vi)
	return res, vi, nil
}
