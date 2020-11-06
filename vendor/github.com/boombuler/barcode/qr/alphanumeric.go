package qr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/boombuler/barcode/utils"
)

const charSet string = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"

func stringToAlphaIdx(content string) <-chan int {
	result := make(chan int)
	go func() {
		for _, r := range content {
			idx := strings.IndexRune(charSet, r)
			result <- idx
			if idx < 0 {
				break
			}
		}
		close(result)
	}()

	return result
}

func encodeAlphaNumeric(content string, ecl ErrorCorrectionLevel) (*utils.BitList, *versionInfo, error) {

	contentLenIsOdd := len(content)%2 == 1
	contentBitCount := (len(content) / 2) * 11
	if contentLenIsOdd {
		contentBitCount += 6
	}
	vi := findSmallestVersionInfo(ecl, alphaNumericMode, contentBitCount)
	if vi == nil {
		return nil, nil, errors.New("To much data to encode")
	}

	res := new(utils.BitList)
	res.AddBits(int(alphaNumericMode), 4)
	res.AddBits(len(content), vi.charCountBits(alphaNumericMode))

	encoder := stringToAlphaIdx(content)

	for idx := 0; idx < len(content)/2; idx++ {
		c1 := <-encoder
		c2 := <-encoder
		if c1 < 0 || c2 < 0 {
			return nil, nil, fmt.Errorf("\"%s\" can not be encoded as %s", content, AlphaNumeric)
		}
		res.AddBits(c1*45+c2, 11)
	}
	if contentLenIsOdd {
		c := <-encoder
		if c < 0 {
			return nil, nil, fmt.Errorf("\"%s\" can not be encoded as %s", content, AlphaNumeric)
		}
		res.AddBits(c, 6)
	}

	addPaddingAndTerminator(res, vi)

	return res, vi, nil
}
