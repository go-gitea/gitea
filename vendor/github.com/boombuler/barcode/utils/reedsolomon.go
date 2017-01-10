package utils

import (
	"sync"
)

type ReedSolomonEncoder struct {
	gf        *GaloisField
	polynomes []*GFPoly
	m         *sync.Mutex
}

func NewReedSolomonEncoder(gf *GaloisField) *ReedSolomonEncoder {
	return &ReedSolomonEncoder{
		gf, []*GFPoly{NewGFPoly(gf, []int{1})}, new(sync.Mutex),
	}
}

func (rs *ReedSolomonEncoder) getPolynomial(degree int) *GFPoly {
	rs.m.Lock()
	defer rs.m.Unlock()

	if degree >= len(rs.polynomes) {
		last := rs.polynomes[len(rs.polynomes)-1]
		for d := len(rs.polynomes); d <= degree; d++ {
			next := last.Multiply(NewGFPoly(rs.gf, []int{1, rs.gf.ALogTbl[d-1+rs.gf.Base]}))
			rs.polynomes = append(rs.polynomes, next)
			last = next
		}
	}
	return rs.polynomes[degree]
}

func (rs *ReedSolomonEncoder) Encode(data []int, eccCount int) []int {
	generator := rs.getPolynomial(eccCount)
	info := NewGFPoly(rs.gf, data)
	info = info.MultByMonominal(eccCount, 1)
	_, remainder := info.Divide(generator)

	result := make([]int, eccCount)
	numZero := int(eccCount) - len(remainder.Coefficients)
	copy(result[numZero:], remainder.Coefficients)
	return result
}
