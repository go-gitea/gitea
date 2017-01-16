package qr

import (
	"github.com/boombuler/barcode/utils"
)

type errorCorrection struct {
	rs *utils.ReedSolomonEncoder
}

var ec = newErrorCorrection()

func newErrorCorrection() *errorCorrection {
	fld := utils.NewGaloisField(285, 256, 0)
	return &errorCorrection{utils.NewReedSolomonEncoder(fld)}
}

func (ec *errorCorrection) calcECC(data []byte, eccCount byte) []byte {
	dataInts := make([]int, len(data))
	for i := 0; i < len(data); i++ {
		dataInts[i] = int(data[i])
	}
	res := ec.rs.Encode(dataInts, int(eccCount))
	result := make([]byte, len(res))
	for i := 0; i < len(res); i++ {
		result[i] = byte(res[i])
	}
	return result
}
